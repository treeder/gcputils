package gcputils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/logging"
	"github.com/treeder/gotils/v2"
	"google.golang.org/api/option"
)

var (
	std        *log.Logger
	onGCE      bool
	onCloudRun bool
	component  string
	clients    *clientWrapper
)

const (
	traceHeader = "X-Cloud-Trace-Context"
)

// Printer common interface
type Printer interface {
	Print(v ...interface{})
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

// Fielder methods for adding structured fields
type Fielder interface {
	// F adds structured key/value pairs which will show up nicely in Cloud Logging.
	// Typically use this on the same line as your Printx()
	F(string, interface{}) Line
	// With clones (unlike F), then adds structured key/value pairs which will show up nicely in Cloud Logging.
	// Use this one if you plan on passing this along to other functions or setting global fields.
	With(string, interface{}) Line

	WithTrace(r *http.Request) Line
}

// Leveler methods to set levels on loggers
type Leveler interface {
	// Debug returns a new logger with Debug severity
	Debug() Line
	// Info returns a new logger with INFO severity
	Info() Line
	// Error returns a new logger with ERROR severity
	Error() Line
}

// Line is the main interface returned from most functions
type Line interface {
	Fielder
	Printer
	Leveler
	Logf(ctx context.Context, severity, format string, a ...interface{})
	Log(ctx context.Context, severity string, a ...interface{})
}

func init() {
	// no prefix, no timestamp
	std = log.New(os.Stderr, "", 0)
	clients = &clientWrapper{}
	onGCE = metadata.OnGCE()
	if onGCE {
		x, _ := metadata.InstanceAttributes()
		fmt.Printf("InstanceAttributes: %+v\n", x)
		s, _ := metadata.InstanceID()
		fmt.Printf("InstanceID: %+v\n", s)
		s, _ = metadata.InstanceName()
		fmt.Printf("InstanceName: %+v\n", s)
		x, _ = metadata.InstanceTags()
		fmt.Printf("InstanceTags: %+v\n", x)
		s, _ = metadata.Zone()
		fmt.Printf("InstanceZone: %+v\n", s)
		// From what I can see, instanceName is empty if on cloud run (instanceID used to be empty)
		// On GCE, there are a couple of instance tags ([http-server https-server]) and instance attributes which appear to also be empty on cloud run
		s, _ = metadata.InstanceName()
		if s == "" {
			onCloudRun = true
		}
	}
}

// InitLogging you must call this to initialize the logging and error reporting clients.
// Not required if using cloud run.
// Call defer x.Close() on the returned closer to ensure logs get flushed.
func InitLogging(ctx context.Context, projectID string, opts []option.ClientOption) (io.Closer, error) {
	clients.projectID = projectID
	var err error
	if onGCE {
		if !onCloudRun {
			clients.logClient, err = logging.NewClient(ctx, projectID)
			if err != nil {
				return clients, fmt.Errorf("error creating google cloud logger: %v", err)
			}
			clients.logger = clients.logClient.Logger("goapp")
			// Setup error reporting and logging clients
			// looks like this isn't required anymore, it will automatically do it with proper logging:
			// https://cloud.google.com/error-reporting/docs/setup/compute-engine#go
			// clients.errorClient, err = errorreporting.NewClient(context.Background(), projectID, errorreporting.Config{
			// 	// ServiceName: "MyService",
			// 	OnError: func(err error) {
			// 		log.Printf("stackdriver: Could not log error: %v", err)
			// 	},
			// }, opts...)
			// if err != nil {
			// 	return clients, fmt.Errorf("error creating error reporting client: %v", err)
			// }
		}
	}
	return clients, nil
}

type clientWrapper struct {
	projectID string
	logClient *logging.Client
	logger    *logging.Logger
}

func (c *clientWrapper) Close() error {
	if c.logClient != nil {
		c.logClient.Close()
	}
	return nil
}

// SetComponent Stackdriver Log Viewer allows filtering and display of this as `jsonPayload.component`.
func SetComponent(s string) {
	component = s
}

// Println take a wild guess
func Println(v ...interface{}) {
	l := &line{sev: logging.Info}
	l.Println(v...)
}

// Print take a wild guess
func Print(v ...interface{}) {
	l := &line{sev: logging.Info}
	l.Print(v...)
}

// Printf take a wild guess
func Printf(format string, v ...interface{}) {
	l := &line{sev: logging.Info}
	l.Printf(format, v...)
}

// P returns a new logger with the provided severity
func P(sev string) Line {
	return &line{sev: logging.ParseSeverity(sev)}
}

// Debug returns a new logger with DEBUG severity
func Debug() Line {
	return &line{sev: logging.Debug}
}

// Info returns a new logger with INFO severity
func Info() Line {
	return &line{sev: logging.Info}
}

func NewLogger() Line {
	return Info()
}

// Error returns a new logger with ERROR severity
func Error() Line {
	return &line{sev: logging.Error}
}

// Errorf will log an error (if it hasn't already been logged) and return an error as if fmt.Errorf was called
func Errorf(format string, v ...interface{}) error {
	l := &line{sev: logging.Error}
	return l.Errorf(format, v...)
}

// Err will log an error (if it hasn't already been logged) and return the same error
func Err(err error) error {
	l := &line{sev: logging.Error}
	return l.Err(err)
}

// With returns a new logger with the fields passed in
func With(key string, value interface{}) Line {
	return F(key, value)
}

// F see line.F()
func F(key string, value interface{}) Line {
	l := &line{sev: logging.Info}
	return l.F(key, value)
}

type line struct {
	sev    logging.Severity
	fields map[string]interface{}
	trace  string
	stack  []runtime.Frame
}

// F adds structured key/value pairs which will show up nicely in Cloud Logging.
// Typically use this on the same line as your Printx()
func (l *line) F(key string, value interface{}) Line {
	if l.fields == nil {
		l.fields = map[string]interface{}{}
	}
	l.fields[key] = value
	return l
}

// With clones (unlike F), then adds structured key/value pairs which will show up nicely in Cloud Logging.
// Use this one if you plan on passing this along to other functions or setting global fields.
func (l *line) With(key string, value interface{}) Line {
	l2 := l.clone()
	l2.fields[key] = value
	return l2
}

func (l *line) clone() *line {
	l2 := *l
	l3 := &l2
	l3.fields = map[string]interface{}{}
	for k, v := range l.fields {
		l3.fields[k] = v
	}
	return l3
}

// Printf prints to the appropriate destination
// Arguments are handled in the manner of fmt.Printf.
func (l *line) Printf(format string, v ...interface{}) {
	print(l, fmt.Sprintf(format, v...), "", v...)
}

// Println prints to the appropriate destination
// Arguments are handled in the manner of fmt.Println.
func (l *line) Println(v ...interface{}) {
	print(l, fmt.Sprintln(v...), "", v...)
}

// Print prints to the appropriate destination
// Arguments are handled in the manner of fmt.Print.
func (l *line) Print(v ...interface{}) {
	print(l, fmt.Sprint(v...), "", v...)
}

func (l *line) Debug() Line {
	l2 := l.clone()
	l2.sev = logging.Debug
	return l2
}

func (l *line) Info() Line {
	l2 := l.clone()
	l2.sev = logging.Info
	return l2
}

func (l *line) Error() Line {
	l2 := l.clone()
	l2.sev = logging.Error
	return l2
}

// gotils thing
func (l *line) Logf(ctx context.Context, severity, format string, a ...interface{}) {
	l = l.clone()
	l.sev = logging.ParseSeverity(severity)
	printCtx(ctx, l, format, a...)
}
func (l *line) Log(ctx context.Context, severity string, a ...interface{}) {
	l = l.clone()
	l.sev = logging.ParseSeverity(severity)
	printCtx(ctx, l, fmt.Sprintln(a...))
}

// sentinel error so we don't log the same thing twice
// var logged = errors.New("already logged")

type loggedError struct {
	err error
}

func (e *loggedError) Error() string { return e.err.Error() }
func (e *loggedError) Unwrap() error { return e.err }

// Errorf will log an error (if it hasn't already been logged) and return an error as if fmt.Errorf was called
func (l *line) Errorf(format string, v ...interface{}) error {
	e2 := fmt.Errorf(format, v...)
	// looping through operands in case user is using %w and we already logged the error
	for _, x := range v {
		switch y := x.(type) {
		case error:
			var e *loggedError
			if errors.As(y, &e) {
				return e2
			}
		}
	}
	return l.Err(e2)
}

// Err will log an error (if it hasn't already been logged) and return the same error
func (l *line) Err(err error) error {
	var e *loggedError
	if errors.As(err, &e) {
		return err
	}
	l2 := l.clone()
	l2.sev = logging.Error
	l.Print(err)
	return &loggedError{err}
}

// WithTrace adds tracing info which Cloud Logging uses to correlate logs related to a particular request
func (l *line) WithTrace(r *http.Request) Line {
	var trace string
	if clients.projectID != "" { // should we log an error here since this won't work without it. "Must call InitLogging"
		traceHeader := r.Header.Get("X-Cloud-Trace-Context")
		traceParts := strings.Split(traceHeader, "/")
		if len(traceParts) > 0 && len(traceParts[0]) > 0 {
			trace = fmt.Sprintf("projects/%s/traces/%s", clients.projectID, traceParts[0])
		}
	}
	l2 := *l
	l2.trace = trace
	return &l2
}

// WithTrace adds tracing info which Cloud Logging uses to correlate logs related to a particular request.WithTrace
// This is for use in conjuction with gotils contextual errors, whereas the other function with the same name
// is used for logging.
func WithTrace(ctx context.Context, r *http.Request) context.Context {
	var trace string
	if clients.projectID != "" { // should we log an error here since this won't work without it. "Must call InitLogging"
		traceHeader := r.Header.Get(traceHeader)
		traceParts := strings.Split(traceHeader, "/")
		if len(traceParts) > 0 && len(traceParts[0]) > 0 {
			trace = fmt.Sprintf("projects/%s/traces/%s", clients.projectID, traceParts[0])
		}
	}
	return gotils.With(ctx, traceHeader, trace)
}

func printCtx(ctx context.Context, line *line, format string, a ...interface{}) {
	// Newer experiment based on this: https://github.com/treeder/gotils/issues/2
	// looping through operands in case user is using %w and we already logged the error
	stack := ""
	for _, x := range a {
		// this can work instead of switch too:
		// _, ok := x.(error)
		// if ok {
		// 	fmt.Println("it's an error")
		// }
		switch y := x.(type) {
		case error:
			// fmt.Println("is error")
			var stacked gotils.FullStacked
			if errors.As(y, &stacked) {
				// fmt.Printf("IS FULL STACKED\n")
				line = line.clone()
				line.sev = logging.Error
				// then we'll output all the good stuff
				line.stack = stacked.Stack()
				stack = gotils.StackToString(line.stack)
				line.fields = stacked.Fields()
				if line.fields != nil {
					tr := line.fields[traceHeader]
					if tr != nil {
						line.trace = tr.(string)
					}
				}
			}

		}
	}
	if stack == "" && line.sev >= logging.Error {
		// buf := make([]byte, 1<<16) // 65536 - seems kinda big?
		// i := runtime.Stack(buf, false)
		// stack = string(buf[0:i])
		stack = gotils.StackToString(gotils.TakeStacktrace())
	}
	print3(ctx, line, fmt.Sprintf(format, a...), stack, "")
}

func print(line *line, message, suffix string, args ...interface{}) {
	// Newer experiment based on this: https://github.com/treeder/gotils/issues/2
	// looping through operands in case user is using %w and we already logged the error
	stack := ""
	for _, x := range args {
		// this can work instead of switch too:
		// _, ok := x.(error)
		// if ok {
		// 	fmt.Println("it's an error")
		// }
		switch y := x.(type) {
		case error:
			// fmt.Println("is error")
			var stacked gotils.FullStacked
			if errors.As(y, &stacked) {
				// fmt.Printf("IS FULL STACKED\n")
				line = line.clone()
				line.sev = logging.Error
				// then we'll output all the good stuff
				stack = gotils.StackToString(stacked.Stack())
				line.fields = stacked.Fields()
				if line.fields != nil {
					tr := line.fields[traceHeader]
					if tr != nil {
						line.trace = tr.(string)
					}
				}
			}

		}
	}
	if stack == "" && line.sev >= logging.Error {
		// buf := make([]byte, 1<<16) // 65536 - seems kinda big?
		// i := runtime.Stack(buf, false)
		// stack = string(buf[0:i])
		stack = gotils.StackToString(gotils.TakeStacktrace())
	}
	print2(line, message, stack, suffix)
}

func print2(line *line, message, stack, suffix string) {
	print3(nil, line, message, stack, suffix)
}

func print3(ctx context.Context, line *line, message, stack, suffix string) {
	sev := line.sev
	if ctx != nil {
		// todo: merge fields from gotils context fields
		// todo: do the same for trace (gotils.withtrace)
		gotilsFields := gotils.Fields(ctx)
		if gotilsFields != nil {
			if line.fields == nil {
				line.fields = map[string]interface{}{}
			}
			for k, v := range gotilsFields {
				// which one takes precedence?
				_, ok := line.fields[k]
				if !ok {
					line.fields[k] = v
				}
			}
		}
	}
	if onGCE {
		msg := message
		if stack != "" {
			msg += "\n" + stack
		}

		if onCloudRun {
			// this will automatically make an error in error reporting
			std.Println(Entry{
				Severity:  sev.String(),
				Message:   msg,
				Component: component,
				Trace:     line.trace, // see https://cloud.google.com/run/docs/logging#writing_structured_logs
				Fields:    line.fields,
			})
			return
		}
		// regular GCE, so using the APIs
		if clients.logger == nil {
			// InitLogging wasn't called, so printing to console
			// todo: Maybe print a message that user should call InitLogging?
			toConsole(ctx, line, message)
			return
		}
		payload := map[string]interface{}{"message": msg}
		if line.fields != nil {
			for k, v := range line.fields {
				payload[k] = v
			}
		}

		clients.logger.Log(logging.Entry{
			Severity: sev,
			// Payload:  "something terrible happened!",
			Payload: payload,
		})
		// lg.Flush()
		return
	}
	// now just regular console
	toConsole(ctx, line, message)
}

func toConsole(ctx context.Context, line *line, message string) {
	gotils.PrintMFS(ctx, line.sev.String(), message, line.fields, line.stack)
}

func shouldSkip(s string) bool {
	// fmt.Println("should skip: ", s)
	if strings.HasPrefix(s, "github.com/treeder/gcputils") {
		return true
	}
	return false
}

type arbFields map[string]interface{}

// Entry defines a log entry.
type Entry struct {
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
	Trace    string `json:"logging.googleapis.com/trace,omitempty"`

	// Stackdriver Log Viewer allows filtering and display of this as `jsonPayload.component`.
	Component string `json:"component,omitempty"`

	Fields map[string]interface{}
}

// added this so we could add arbitrary fields too
func (e Entry) flatten(m map[string]interface{}) {
	m["message"] = e.Message
	m["severity"] = e.Severity
	if e.Trace != "" {
		m["logging.googleapis.com/trace"] = e.Trace
	}
	if e.Component != "" {
		m["component"] = e.Component
	}
	if e.Fields != nil {
		for k, v := range e.Fields {
			m[k] = v
		}
	}
}

// String renders an entry structure to the JSON format expected by Stackdriver.
func (e Entry) String() string {
	if e.Severity == "" {
		e.Severity = "INFO"
	}
	m := map[string]interface{}{}
	e.flatten(m)
	out, err := json.Marshal(m)
	if err != nil {
		log.Printf("json.Marshal: %v", err)
	}
	return string(out)
}
