package gcputils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"

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
	AddField(string, interface{}) Line
}

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

// Printf prints to the appropriate destination
// Arguments are handled in the manner of fmt.Printf.
func (l *line) Printf(format string, v ...interface{}) {
	print(l.sev, fmt.Sprintf(format, v...))
}

// Println prints to the appropriate destination
// Arguments are handled in the manner of fmt.Println.
func (l *line) Println(v ...interface{}) {
	print(l.sev, fmt.Sprintln(v...))
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

func print(sev logging.Severity, message string) {
	msg := message
	// sev := logging.ParseSeverity(severity)
	if sev >= logging.Error {
		buf := make([]byte, 1<<16) // 65536 - seems kinda big?
		i := runtime.Stack(buf, false)
		msg = msg + "\n" + string(buf[0:i])
	}
	if onGCE {
		if onCloudRun {
			// this will automatically make an error in error reporting
			std.Println(Entry{
				Severity:  sev.String(),
				Message:   msg,
				Component: component,
				// Trace:     trace, // see https://cloud.google.com/run/docs/logging#writing_structured_logs
			})
			return
		}
		// regular GCE, so using the APIs
		clients.logger.Log(logging.Entry{
			Severity: sev,
			// Payload:  "something terrible happened!",
			Payload: map[string]interface{}{"message": msg},
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
	fmt.Println(msg)
}

// Entry defines a log entry.
type Entry struct {
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
	Trace    string `json:"logging.googleapis.com/trace,omitempty"`

	// Stackdriver Log Viewer allows filtering and display of this as `jsonPayload.component`.
	Component string `json:"component,omitempty"`
}

// String renders an entry structure to the JSON format expected by Stackdriver.
func (e Entry) String() string {
	if e.Severity == "" {
		e.Severity = "INFO"
	}
	out, err := json.Marshal(e)
	if err != nil {
		log.Printf("json.Marshal: %v", err)
	}
	return string(out)
}
