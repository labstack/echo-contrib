/*
Package jaegertracing provides middleware to Opentracing using Jaeger.

Example:
```
package main
import (
    "github.com/labstack/echo-contrib/jaegertracing"
    "github.com/labstack/echo/v4"
)
func main() {
    e := echo.New()
    // Enable tracing middleware
    c := jaegertracing.New(e, nil)
    defer c.Close()

    e.Logger.Fatal(e.Start(":1323"))
}
```
*/
package jaegertracing

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"runtime"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go/config"
)

const defaultComponentName = "echo/v4"

type (
	// TraceConfig defines the config for Trace middleware.
	TraceConfig struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// OpenTracing Tracer instance which should be got before
		Tracer opentracing.Tracer

		// ComponentName used for describing the tracing component name
		ComponentName string

		// add req body & resp body to tracing tags
		IsBodyDump bool
	}
)

var (
	// DefaultTraceConfig is the default Trace middleware config.
	DefaultTraceConfig = TraceConfig{
		Skipper:       middleware.DefaultSkipper,
		ComponentName: defaultComponentName,
		IsBodyDump:    false,
	}
)

// New creates an Opentracing tracer and attaches it to Echo middleware.
// Returns Closer do be added to caller function as `defer closer.Close()`
func New(e *echo.Echo, skipper middleware.Skipper) io.Closer {
	// Add Opentracing instrumentation
	defcfg := config.Configuration{
		ServiceName: "echo-tracer",
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans:            true,
			BufferFlushInterval: 1 * time.Second,
		},
	}
	cfg, err := defcfg.FromEnv()
	if err != nil {
		panic("Could not parse Jaeger env vars: " + err.Error())
	}
	tracer, closer, err := cfg.NewTracer()
	if err != nil {
		panic("Could not initialize jaeger tracer: " + err.Error())
	}

	opentracing.SetGlobalTracer(tracer)
	e.Use(TraceWithConfig(TraceConfig{
		Tracer:  tracer,
		Skipper: skipper,
	}))
	return closer
}

// Trace returns a Trace middleware.
// Trace middleware traces http requests and reporting errors.
func Trace(tracer opentracing.Tracer) echo.MiddlewareFunc {
	c := DefaultTraceConfig
	c.Tracer = tracer
	c.ComponentName = defaultComponentName
	return TraceWithConfig(c)
}

// TraceWithConfig returns a Trace middleware with config.
// See: `Trace()`.
func TraceWithConfig(config TraceConfig) echo.MiddlewareFunc {
	if config.Tracer == nil {
		panic("echo: trace middleware requires opentracing tracer")
	}
	if config.Skipper == nil {
		config.Skipper = middleware.DefaultSkipper
	}
	if config.ComponentName == "" {
		config.ComponentName = defaultComponentName
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			req := c.Request()
			opname := "HTTP " + req.Method + " URL: " + c.Path()
			var sp opentracing.Span
			tr := config.Tracer
			if ctx, err := tr.Extract(opentracing.HTTPHeaders,
				opentracing.HTTPHeadersCarrier(req.Header)); err != nil {
				sp = tr.StartSpan(opname)
			} else {
				sp = tr.StartSpan(opname, ext.RPCServerOption(ctx))
			}

			ext.HTTPMethod.Set(sp, req.Method)
			ext.HTTPUrl.Set(sp, req.URL.String())
			ext.Component.Set(sp, config.ComponentName)

			// Dump request & response body
			resBody := new(bytes.Buffer)
			if config.IsBodyDump {
				// request
				reqBody := []byte{}
				if c.Request().Body != nil { // Read
					reqBody, _ = ioutil.ReadAll(c.Request().Body)
					sp.SetTag("http.req.body", string(reqBody))
				}

				req.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody)) // Reset

				// response
				mw := io.MultiWriter(c.Response().Writer, resBody)
				writer := &bodyDumpResponseWriter{Writer: mw, ResponseWriter: c.Response().Writer}
				c.Response().Writer = writer
			}

			req = req.WithContext(opentracing.ContextWithSpan(req.Context(), sp))
			c.SetRequest(req)

			var err error
			defer func() {
				committed := c.Response().Committed
				status := c.Response().Status

				if err != nil {
					var httpError *echo.HTTPError
					if errors.As(err, &httpError) {
						if httpError.Code != 0 {
							status = httpError.Code
						}
						sp.SetTag("error.message", httpError.Message)
					} else {
						sp.SetTag("error.message", err.Error())
					}
					if status == http.StatusOK {
						// this is ugly workaround for cases when httpError.code == 0 or error was not httpError and status
						// in request was 200 (OK). In these cases replace status with something that represents an error
						// it could be that error handlers or middlewares up in chain will output different status code to
						// client. but at least we send something better than 200 to jaeger
						status = http.StatusInternalServerError
					}
				}

				ext.HTTPStatusCode.Set(sp, uint16(status))
				if status >= http.StatusInternalServerError || !committed {
					ext.Error.Set(sp, true)
				}

				// Dump response body
				if config.IsBodyDump {
					sp.SetTag("http.resp.body", string(resBody.Bytes()))
				}

				sp.Finish()
			}()
			err = next(c)
			return err
		}
	}
}

// TraceFunction wraps funtion with opentracing span adding tags for the function name and caller details
func TraceFunction(ctx echo.Context, fn interface{}, params ...interface{}) (result []reflect.Value) {
	// Get function name
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	// Create child span
	parentSpan := opentracing.SpanFromContext(ctx.Request().Context())
	sp := opentracing.StartSpan(
		"Function - "+name,
		opentracing.ChildOf(parentSpan.Context()))
	defer sp.Finish()

	sp.SetTag("function", name)

	// Get caller function name, file and line
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	callerDetails := fmt.Sprintf("%s - %s#%d", frame.Function, frame.File, frame.Line)
	sp.SetTag("caller", callerDetails)

	// Check params and call function
	f := reflect.ValueOf(fn)
	if f.Type().NumIn() != len(params) {
		e := fmt.Sprintf("Incorrect number of parameters calling wrapped function %s", name)
		panic(e)
	}
	inputs := make([]reflect.Value, len(params))
	for k, in := range params {
		inputs[k] = reflect.ValueOf(in)
	}
	return f.Call(inputs)
}

// CreateChildSpan creates a new opentracing span adding tags for the span name and caller details.
// User must call defer `sp.Finish()`
func CreateChildSpan(ctx echo.Context, name string) opentracing.Span {
	parentSpan := opentracing.SpanFromContext(ctx.Request().Context())
	sp := opentracing.StartSpan(
		name,
		opentracing.ChildOf(parentSpan.Context()))
	sp.SetTag("name", name)

	// Get caller function name, file and line
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	callerDetails := fmt.Sprintf("%s - %s#%d", frame.Function, frame.File, frame.Line)
	sp.SetTag("caller", callerDetails)

	return sp
}

// NewTracedRequest generates a new traced HTTP request with opentracing headers injected into it
func NewTracedRequest(method string, url string, body io.Reader, span opentracing.Span) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		panic(err.Error())
	}

	ext.SpanKindRPCClient.Set(span)
	ext.HTTPUrl.Set(span, url)
	ext.HTTPMethod.Set(span, method)
	span.Tracer().Inject(span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))

	return req, err
}
