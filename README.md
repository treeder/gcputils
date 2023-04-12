# GCP Utils

Little Go helpers for Google Cloud Platform. 

This is where I experiment with Go things specifically for Google Cloud.

## Logging

Why another lib?  I got annoyed with trying to get Zap and others to work properly with Stack Driver (tip: they don't). 
On top of that, the various Google services handle logging differently which also adds to the annoying level. 

So I thought I'd just make one that works with any Google Cloud services.

Here's what I found: 

* If you are using GCE and writing to stdout/stderr, you probably won't/can't get it right, even if you use the Docker gcplogs logging driver. You will most likely spend way too much time trying to figure this out and you WON'T be able to do it. So don't bother.
* For GCE you must use the logging API so go grab a client and start logging that way. 
* If you are using Cloud Run, you CAN write to stdout/stderr and you CAN get it right. You just need to log in a specific JSON format. 
* If you are using Cloud Run and you write a log with severity ERROR or higher, it will automatically create an incident in error reporting. Awesome! But this is ONLY in Cloud Run. And you must include the stack trace in the message field. Yes, that's right, append a line break and your stack trace to the "message" field in order to get error reporting to pick it up.
* If you want to use Error Reporting on GCE, you have to call it explicitly.

The logging stuff in here handles all those cases.

While I was at it, I thought I'd try to make it more stdlib'ish. Here's how to use it.

This is a `Loggable` for [treeder/gotils](https://github.com/treeder/gotils/) so you really just have to have gotils use this:

On startup:

```go
gotils.SetLoggable(gcputils.NewLogger())
```

Then usage is just gotils usage:

```go
// basic log message
gotils.L(ctx).Info().Println("hi")
// add fields
l := gotils.With("abc", 123)
// then anytime you write a log, those structured fields will be output in the proper format for Google Cloud, or human
// readable when developing locally. 
gotils.L(ctx).Error().Printf("some error: %v", err)
// To keep stack traces and have those logged to google cloud logging, just use this whenever you return an error:
return gotils.C(ctx).Errorf("some error: %w", err)
```
