package zipkintracing

import (
	"fmt"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/middleware/http"
	"github.com/openzipkin/zipkin-go/model"
	"github.com/openzipkin/zipkin-go/propagation/b3"
)

type (

	//Tags func to adds span tags
	Tags func(c echo.Context) map[string]string

	//TraceProxyConfig config for TraceProxyWithConfig
	TraceProxyConfig struct {
		Skipper  middleware.Skipper
		Tracer   *zipkin.Tracer
		SpanTags Tags
	}

	//TraceServerConfig config for TraceServerWithConfig
	TraceServerConfig struct {
		Skipper  middleware.Skipper
		Tracer   *zipkin.Tracer
		SpanTags Tags
	}
)

var (
	//DefaultSpanTags default span tags
	DefaultSpanTags = func(c echo.Context) map[string]string {
		return make(map[string]string)
	}

	//DefaultTraceProxyConfig default config for Trace Proxy
	DefaultTraceProxyConfig = TraceProxyConfig{Skipper: middleware.DefaultSkipper, SpanTags: DefaultSpanTags}

	//DefaultTraceServerConfig default config for Trace Server
	DefaultTraceServerConfig = TraceServerConfig{Skipper: middleware.DefaultSkipper, SpanTags: DefaultSpanTags}
)

// DoHTTP is a http zipkin tracer implementation of HTTPDoer
func DoHTTP(c echo.Context, r *http.Request, client *zipkinhttp.Client) (*http.Response, error) {
	req := r.WithContext(c.Request().Context())
	return client.DoWithAppSpan(req, req.Method)
}

// TraceFunc wraps function call with span so that we can trace time taken by func, eventContext only provided if we want to store trace headers
func TraceFunc(c echo.Context, spanName string, spanTags Tags, tracer *zipkin.Tracer) func() {
	span, _ := tracer.StartSpanFromContext(c.Request().Context(), spanName)
	for key, value := range spanTags(c) {
		span.Tag(key, value)
	}

	finishSpan := func() {
		span.Finish()
	}

	return finishSpan
}

//TraceProxy middleware that traces reverse proxy
func TraceProxy(tracer *zipkin.Tracer) echo.MiddlewareFunc {
	config := DefaultTraceProxyConfig
	config.Tracer = tracer
	return TraceProxyWithConfig(config)
}

// TraceProxyWithConfig middleware that traces reverse proxy
func TraceProxyWithConfig(config TraceProxyConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}
			var parentContext model.SpanContext
			if span := zipkin.SpanFromContext(c.Request().Context()); span != nil {
				parentContext = span.Context()
			}
			span := config.Tracer.StartSpan(fmt.Sprintf("C %s %s", c.Request().Method, "reverse proxy"), zipkin.Parent(parentContext))
			for key, value := range config.SpanTags(c) {
				span.Tag(key, value)
			}
			defer span.Finish()
			ctx := zipkin.NewContext(c.Request().Context(), span)
			c.SetRequest(c.Request().WithContext(ctx))
			b3.InjectHTTP(c.Request())(span.Context())
			nrw := NewResponseWriter(c.Response().Writer)
			if err := next(c); err != nil {
				c.Error(err)
			}
			if nrw.Size() > 0 {
				zipkin.TagHTTPResponseSize.Set(span, strconv.FormatInt(int64(nrw.Size()), 10))
			}
			if nrw.Status() < 200 || nrw.Status() > 299 {
				statusCode := strconv.FormatInt(int64(nrw.Status()), 10)
				zipkin.TagHTTPStatusCode.Set(span, statusCode)
				if nrw.Status() > 399 {
					zipkin.TagError.Set(span, statusCode)
				}
			}
			return nil
		}
	}
}

//TraceServer middleware that traces server calls
func TraceServer(tracer *zipkin.Tracer) echo.MiddlewareFunc {
	config := DefaultTraceServerConfig
	config.Tracer = tracer
	return TraceServerWithConfig(config)
}

// TraceServerWithConfig middleware that traces server calls
func TraceServerWithConfig(config TraceServerConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}
			sc := config.Tracer.Extract(b3.ExtractHTTP(c.Request()))
			span := config.Tracer.StartSpan(fmt.Sprintf("S %s %s", c.Request().Method, c.Request().URL.Path), zipkin.Parent(sc))
			for key, value := range config.SpanTags(c) {
				span.Tag(key, value)
			}
			defer span.Finish()
			ctx := zipkin.NewContext(c.Request().Context(), span)
			c.SetRequest(c.Request().WithContext(ctx))
			nrw := NewResponseWriter(c.Response().Writer)
			if err := next(c); err != nil {
				c.Error(err)
			}

			if nrw.Size() > 0 {
				zipkin.TagHTTPResponseSize.Set(span, strconv.FormatInt(int64(nrw.Size()), 10))
			}
			if nrw.Status() < 200 || nrw.Status() > 299 {
				statusCode := strconv.FormatInt(int64(nrw.Status()), 10)
				zipkin.TagHTTPStatusCode.Set(span, statusCode)
				if nrw.Status() > 399 {
					zipkin.TagError.Set(span, statusCode)
				}
			}
			return nil
		}
	}
}

// StartChildSpan starts a new child span as child of parent span from context
// user must call defer childSpan.Finish()
func StartChildSpan(c echo.Context, spanName string, tracer *zipkin.Tracer) (childSpan zipkin.Span) {
	var parentContext model.SpanContext

	if span := zipkin.SpanFromContext(c.Request().Context()); span != nil {
		parentContext = span.Context()
	}
	childSpan = tracer.StartSpan(spanName, zipkin.Parent(parentContext))
	return childSpan
}
