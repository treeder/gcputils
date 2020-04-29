package gcputils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/errorreporting"
	"cloud.google.com/go/logging"
	"google.golang.org/api/option"
)

var (
	std        *log.Logger
	onGCE      bool
	onCloudRun bool
	component  string
	clients    *clientWrapper
)

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
		// From what I found, instanceID will be empty if on cloud run
		s, _ = metadata.InstanceID()
		fmt.Printf("InstanceID: %+v\n", s)
		if s == "" {
			onCloudRun = true
		}
	}
}

// InitLogging you must call this to initialize the logging and error reporting clients.
// Not required if using cloud run.
// Call defer x.Close() on the returned closer to ensure logs get flushed.
func InitLogging(ctx context.Context, projectID string, opts []option.ClientOption) (io.Closer, error) {
	var err error
	if onGCE {
		if !onCloudRun {
			clients.logClient, err = logging.NewClient(ctx, projectID)
			if err != nil {
				return clients, fmt.Errorf("error creating google cloud logger: %v", err)
			}
			clients.logger = clients.logClient.Logger("goapp")
			// setup error reporting and logging clients
			clients.errorClient, err = errorreporting.NewClient(context.Background(), projectID, errorreporting.Config{
				// ServiceName: "MyService",
				OnError: func(err error) {
					log.Printf("stackdriver: Could not log error: %v", err)
				},
			}, opts...)
			if err != nil {
				return clients, fmt.Errorf("error creating error reporting client: %v", err)
			}
		}
	}
	return clients, nil
}

type clientWrapper struct {
	logClient   *logging.Client
	logger      *logging.Logger
	errorClient *errorreporting.Client
}

func (c *clientWrapper) Close() error {
	if c.logClient != nil {
		c.logClient.Close()
	}
	if c.errorClient != nil {
		c.errorClient.Close()
	}
	return nil
}

// SetComponent Stackdriver Log Viewer allows filtering and display of this as `jsonPayload.component`.
func SetComponent(s string) {
	component = s
}

// Printer common interface
type Printer interface {
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

// Fielder methods for adding structured fields
type Fielder interface {
	F(string, interface{}) Line
}

// Line is the main interface returned from most functions, has Fielder and Printer
type Line interface {
	Fielder
	Printer
}

type line struct {
	sev    logging.Severity
	fields map[string]interface{}
}

func (l *line) AddField(key string, value interface{}) Line {
	if l.fields == nil {
		l.fields = map[string]interface{}{}
	}
	l.fields[key] = value
	return l
}

func (l *line) F(key string, value interface{}) Line {
	return l.AddField(key, value)
}

// Printf prints to the appropriate destination
// Arguments are handled in the manner of fmt.Printf.
func (l *line) Printf(format string, v ...interface{}) {
	print(l, fmt.Sprintf(format, v...), "")
}

// Println prints to the appropriate destination
// Arguments are handled in the manner of fmt.Println.
func (l *line) Println(v ...interface{}) {
	print(l, fmt.Sprint(v...), "\n")
}

func P(sev string) Line {
	return &line{sev: logging.ParseSeverity(sev)}
}
func Info() Line {
	return &line{sev: logging.Info}
}
func Error() Line {
	return &line{sev: logging.Error}
}

func print(line *line, message, suffix string) {
	sev := line.sev
	// sev := logging.ParseSeverity(severity)
	stack := ""
	if sev >= logging.Error {
		// buf := make([]byte, 1<<16) // 65536 - seems kinda big?
		// i := runtime.Stack(buf, false)
		// stack = string(buf[0:i])
		stack = takeStacktrace()
	}
	if onGCE {
		msg := message + "\n" + stack
		if onCloudRun {
			// this will automatically make an error in error reporting
			std.Println(Entry{
				Severity:  sev.String(),
				Message:   msg,
				Component: component,
				// Trace:     trace, // see https://cloud.google.com/run/docs/logging#writing_structured_logs
				Fields: line.fields,
			})
			return
		}
		// regular GCE, so using the APIs
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
		if sev >= logging.Error {
			clients.errorClient.Report(errorreporting.Entry{
				Error: errors.New(message),
				// I think this adds the stack automatically, so not including it here
				// User:  "some user", // TODO: could add this feature in somehow
			})
			// errorClient.Flush()
		}
		return
	}
	// now just regular console
	// add fields to msg
	msg := "\t" + strings.ToUpper(sev.String()) + "\t" + message
	if line.fields != nil {
		// msg += "\n"
		for k, v := range line.fields {
			msg += fmt.Sprintf(" [%v=%v]", k, v)
		}
	}
	msg += "\n"
	msg += stack
	msg += suffix
	log.Println(msg)
}

func takeStacktrace() string {
	buffer := bytes.Buffer{}
	pc := make([]uintptr, 25)
	_ = runtime.Callers(2, pc)
	i := 0
	frames := runtime.CallersFrames(pc)
	for frame, more := frames.Next(); more; frame, more = frames.Next() {
		if shouldSkip(frame.Function) {
			continue
		}

		if i != 0 {
			buffer.WriteByte('\n')
		}
		i++
		buffer.WriteString(frame.Function)
		buffer.WriteRune('\n')
		buffer.WriteRune('\t')
		buffer.WriteString(frame.File)
		buffer.WriteRune(':')
		buffer.WriteString(strconv.Itoa(frame.Line))
	}
	return buffer.String()
}

func shouldSkip(s string) bool {
	fmt.Println("should skip: ", s)
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
