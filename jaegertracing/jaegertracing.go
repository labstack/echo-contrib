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
	"fmt"
	"io"
	"log"
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

		// componentName used for describing the tracing component name
		componentName string
	}
)

var (
	// DefaultTraceConfig is the default Trace middleware config.
	DefaultTraceConfig = TraceConfig{
		Skipper:       middleware.DefaultSkipper,
		componentName: defaultComponentName,
	}
)

// New creates an Opentracing tracer and attaches it to Echo middleware.
// Returns Closer do be added to caller function as `defer closer.Close()`
func New(e *echo.Echo, skipper middleware.Skipper) io.Closer {
	// Add Opentracing instrumentation
	cfg, err := config.FromEnv()
	if err != nil {
		// parsing errors might happen here, such as when we get a string where we expect a number
		log.Fatal("Could not parse Jaeger env vars: %s", err.Error())
	}

	cfg.ServiceName = "echo-tracer"
	cfg.Sampler.Type = "const"
	cfg.Sampler.Param = 1
	cfg.Reporter.LogSpans = true
	cfg.Reporter.BufferFlushInterval = 1 * time.Second

	tracer, closer, err := cfg.NewTracer()
	if err != nil {
		log.Fatal("Could not initialize jaeger tracer: %s", err.Error())
	}

	opentracing.SetGlobalTracer(tracer)
	e.Use(TraceWithConfig(TraceConfig{
		Tracer:  tracer,
		Skipper: skipper,
	}))
	return closer
}

// Trace returns a Trace middleware.
//
// Trace middleware traces http requests and reporting errors.
func Trace(tracer opentracing.Tracer) echo.MiddlewareFunc {
	c := DefaultTraceConfig
	c.Tracer = tracer
	c.componentName = defaultComponentName
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
	if config.componentName == "" {
		config.componentName = defaultComponentName
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
			ext.Component.Set(sp, config.componentName)

			req = req.WithContext(opentracing.ContextWithSpan(req.Context(), sp))
			c.SetRequest(req)

			defer func() {
				status := c.Response().Status
				committed := c.Response().Committed
				ext.HTTPStatusCode.Set(sp, uint16(status))
				if status >= http.StatusInternalServerError || !committed {
					ext.Error.Set(sp, true)
				}
				sp.Finish()
			}()
			return next(c)
		}
	}
}

// TraceFunction wraps funtion with opentracing span adding tags for the function name and caller details
func TraceFunction(ctx echo.Context, fn interface{}, params ...interface{}) (result []reflect.Value) {
	f := reflect.ValueOf(fn)
	if f.Type().NumIn() != len(params) {
		panic("incorrect number of parameters!")
	}
	inputs := make([]reflect.Value, len(params))
	for k, in := range params {
		inputs[k] = reflect.ValueOf(in)
	}
	pc, file, no, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	parentSpan := opentracing.SpanFromContext(ctx.Request().Context())
	sp := opentracing.StartSpan(
		"Function - "+name,
		opentracing.ChildOf(parentSpan.Context()))
	(opentracing.Tag{Key: "function", Value: name}).Set(sp)
	if ok {
		callerDetails := fmt.Sprintf("%s - %s#%d", details.Name(), file, no)
		(opentracing.Tag{Key: "caller", Value: callerDetails}).Set(sp)

	}
	defer sp.Finish()
	return f.Call(inputs)
}

// CreateChildSpan creates a new opentracing span adding tags for the span name and caller details.
// User must call defer `sp.Finish()`
func CreateChildSpan(ctx echo.Context, name string) opentracing.Span {
	pc, file, no, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	parentSpan := opentracing.SpanFromContext(ctx.Request().Context())
	sp := opentracing.StartSpan(
		name,
		opentracing.ChildOf(parentSpan.Context()))
	(opentracing.Tag{Key: "name", Value: name}).Set(sp)
	if ok {
		callerDetails := fmt.Sprintf("%s - %s#%d", details.Name(), file, no)
		(opentracing.Tag{Key: "caller", Value: callerDetails}).Set(sp)

	}
	return sp
}
