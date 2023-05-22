package echoprometheus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCustomRegistryMetrics(t *testing.T) {
	e := echo.New()

	customRegistry := prometheus.NewRegistry()
	e.Use(NewMiddlewareWithConfig(MiddlewareConfig{Registerer: customRegistry}))
	e.GET("/metrics", NewHandlerWithConfig(HandlerConfig{Gatherer: customRegistry}))

	assert.Equal(t, http.StatusNotFound, request(e, "/ping?test=1"))

	s, code := requestBody(e, "/metrics")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, s, `echo_request_duration_seconds_count{code="404",host="example.com",method="GET",url="/ping"} 1`)
}

func TestDefaultRegistryMetrics(t *testing.T) {
	e := echo.New()

	e.Use(NewMiddleware("myapp"))
	e.GET("/metrics", NewHandler())

	assert.Equal(t, http.StatusNotFound, request(e, "/ping?test=1"))

	s, code := requestBody(e, "/metrics")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, s, `myapp_request_duration_seconds_count{code="404",host="example.com",method="GET",url="/ping"} 1`)

	unregisterDefaults("myapp")
}

func TestPrometheus_Buckets(t *testing.T) {
	e := echo.New()

	customRegistry := prometheus.NewRegistry()
	e.Use(NewMiddlewareWithConfig(MiddlewareConfig{Registerer: customRegistry}))
	e.GET("/metrics", NewHandlerWithConfig(HandlerConfig{Gatherer: customRegistry}))

	assert.Equal(t, http.StatusNotFound, request(e, "/ping"))

	body, code := requestBody(e, "/metrics")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `echo_request_duration_seconds_bucket{code="404",host="example.com",method="GET",url="/ping",le="0.005"}`, "duration should have time bucket (like, 0.005s)")
	assert.NotContains(t, body, `echo_request_duration_seconds_bucket{code="404",host="example.com",method="GET",url="/ping",le="512000"}`, "duration should NOT have a size bucket (like, 512K)")
	assert.Contains(t, body, `echo_request_size_bytes_bucket{code="404",host="example.com",method="GET",url="/ping",le="1024"}`, "request size should have a 1024k (size) bucket")
	assert.NotContains(t, body, `echo_request_size_bytes_bucket{code="404",host="example.com",method="GET",url="/ping",le="0.005"}`, "request size should NOT have time bucket (like, 0.005s)")
	assert.Contains(t, body, `echo_response_size_bytes_bucket{code="404",host="example.com",method="GET",url="/ping",le="1024"}`, "response size should have a 1024k (size) bucket")
	assert.NotContains(t, body, `echo_response_size_bytes_bucket{code="404",host="example.com",method="GET",url="/ping",le="0.005"}`, "response size should NOT have time bucket (like, 0.005s)")
}

func TestMiddlewareConfig_Skipper(t *testing.T) {
	e := echo.New()

	customRegistry := prometheus.NewRegistry()
	e.Use(NewMiddlewareWithConfig(MiddlewareConfig{
		Skipper: func(c echo.Context) bool {
			hasSuffix := strings.HasSuffix(c.Path(), "ignore")
			return hasSuffix
		},
		Registerer: customRegistry,
	}))

	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})
	e.GET("/test_ignore", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	assert.Equal(t, http.StatusNotFound, request(e, "/ping"))
	assert.Equal(t, http.StatusOK, request(e, "/test"))
	assert.Equal(t, http.StatusOK, request(e, "/test_ignore"))

	out := &bytes.Buffer{}
	assert.NoError(t, WriteGatheredMetrics(out, customRegistry))

	body := out.String()
	assert.Contains(t, body, `echo_request_duration_seconds_count{code="200",host="example.com",method="GET",url="/test"} 1`)
	assert.Contains(t, body, `echo_request_duration_seconds_count{code="404",host="example.com",method="GET",url="/ping"} 1`)
	assert.Contains(t, body, `echo_request_duration_seconds_count{code="404",host="example.com",method="GET",url="/ping"} 1`)
	assert.NotContains(t, body, `test_ignore`) // because we skipped
}

func TestMetricsForErrors(t *testing.T) {
	e := echo.New()
	customRegistry := prometheus.NewRegistry()
	e.Use(NewMiddlewareWithConfig(MiddlewareConfig{
		Skipper: func(c echo.Context) bool {
			return strings.HasSuffix(c.Path(), "ignore")
		},
		Subsystem:  "myapp",
		Registerer: customRegistry,
	}))
	e.GET("/metrics", NewHandlerWithConfig(HandlerConfig{Gatherer: customRegistry}))

	e.GET("/handler_for_ok", func(c echo.Context) error {
		return c.JSON(http.StatusOK, "OK")
	})
	e.GET("/handler_for_nok", func(c echo.Context) error {
		return c.JSON(http.StatusConflict, "NOK")
	})
	e.GET("/handler_for_error", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusBadGateway, "BAD")
	})

	assert.Equal(t, http.StatusOK, request(e, "/handler_for_ok"))
	assert.Equal(t, http.StatusConflict, request(e, "/handler_for_nok"))
	assert.Equal(t, http.StatusConflict, request(e, "/handler_for_nok"))
	assert.Equal(t, http.StatusBadGateway, request(e, "/handler_for_error"))

	body, code := requestBody(e, "/metrics")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, fmt.Sprintf("%s_requests_total", "myapp"))
	assert.Contains(t, body, `myapp_requests_total{code="200",host="example.com",method="GET",url="/handler_for_ok"} 1`)
	assert.Contains(t, body, `myapp_requests_total{code="409",host="example.com",method="GET",url="/handler_for_nok"} 2`)
	assert.Contains(t, body, `myapp_requests_total{code="502",host="example.com",method="GET",url="/handler_for_error"} 1`)
}

func TestMiddlewareConfig_LabelFuncs(t *testing.T) {
	e := echo.New()
	customRegistry := prometheus.NewRegistry()
	e.Use(NewMiddlewareWithConfig(MiddlewareConfig{
		LabelFuncs: map[string]LabelValueFunc{
			"scheme": func(c echo.Context, err error) string { // additional custom label
				return c.Scheme()
			},
			"method": func(c echo.Context, err error) string { // overrides default 'method' label value
				return "overridden_" + c.Request().Method
			},
		},
		Registerer: customRegistry,
	}))
	e.GET("/metrics", NewHandlerWithConfig(HandlerConfig{Gatherer: customRegistry}))

	e.GET("/ok", func(c echo.Context) error {
		return c.JSON(http.StatusOK, "OK")
	})

	assert.Equal(t, http.StatusOK, request(e, "/ok"))

	body, code := requestBody(e, "/metrics")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `echo_request_duration_seconds_count{code="200",host="example.com",method="overridden_GET",scheme="http",url="/ok"} 1`)
}

func TestMiddlewareConfig_HistogramOptsFunc(t *testing.T) {
	e := echo.New()
	customRegistry := prometheus.NewRegistry()
	e.Use(NewMiddlewareWithConfig(MiddlewareConfig{
		HistogramOptsFunc: func(opts prometheus.HistogramOpts) prometheus.HistogramOpts {
			if opts.Name == "request_duration_seconds" {
				opts.ConstLabels = prometheus.Labels{"my_const": "123"}
			}
			return opts
		},
		Registerer: customRegistry,
	}))
	e.GET("/metrics", NewHandlerWithConfig(HandlerConfig{Gatherer: customRegistry}))

	e.GET("/ok", func(c echo.Context) error {
		return c.JSON(http.StatusOK, "OK")
	})

	assert.Equal(t, http.StatusOK, request(e, "/ok"))

	body, code := requestBody(e, "/metrics")
	assert.Equal(t, http.StatusOK, code)

	// has const label
	assert.Contains(t, body, `echo_request_duration_seconds_count{code="200",host="example.com",method="GET",my_const="123",url="/ok"} 1`)
	// does not have const label
	assert.Contains(t, body, `echo_request_size_bytes_count{code="200",host="example.com",method="GET",url="/ok"} 1`)
}

func TestMiddlewareConfig_CounterOptsFunc(t *testing.T) {
	e := echo.New()
	customRegistry := prometheus.NewRegistry()
	e.Use(NewMiddlewareWithConfig(MiddlewareConfig{
		CounterOptsFunc: func(opts prometheus.CounterOpts) prometheus.CounterOpts {
			if opts.Name == "requests_total" {
				opts.ConstLabels = prometheus.Labels{"my_const": "123"}
			}
			return opts
		},
		Registerer: customRegistry,
	}))
	e.GET("/metrics", NewHandlerWithConfig(HandlerConfig{Gatherer: customRegistry}))

	e.GET("/ok", func(c echo.Context) error {
		return c.JSON(http.StatusOK, "OK")
	})

	assert.Equal(t, http.StatusOK, request(e, "/ok"))

	body, code := requestBody(e, "/metrics")
	assert.Equal(t, http.StatusOK, code)

	// has const label
	assert.Contains(t, body, `echo_requests_total{code="200",host="example.com",method="GET",my_const="123",url="/ok"} 1`)
	// does not have const label
	assert.Contains(t, body, `echo_request_size_bytes_count{code="200",host="example.com",method="GET",url="/ok"} 1`)
}

func TestMiddlewareConfig_AfterNextFuncs(t *testing.T) {
	e := echo.New()

	customRegistry := prometheus.NewRegistry()
	customCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "custom_requests_total",
			Help: "How many HTTP requests processed, partitioned by status code and HTTP method.",
		},
	)
	if err := customRegistry.Register(customCounter); err != nil {
		t.Fatal(err)
	}

	e.Use(NewMiddlewareWithConfig(MiddlewareConfig{
		AfterNext: func(c echo.Context, err error) {
			customCounter.Inc() // use our custom metric in middleware
		},
		Registerer: customRegistry,
	}))
	e.GET("/metrics", NewHandlerWithConfig(HandlerConfig{Gatherer: customRegistry}))

	e.GET("/ok", func(c echo.Context) error {
		return c.JSON(http.StatusOK, "OK")
	})

	assert.Equal(t, http.StatusOK, request(e, "/ok"))

	body, code := requestBody(e, "/metrics")
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, body, `custom_requests_total 1`)
}

func TestRunPushGatewayGatherer(t *testing.T) {
	receivedMetrics := false
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMetrics = true
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("OK"))
	}))
	defer svr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	config := PushGatewayConfig{
		PushGatewayURL: svr.URL,
		PushInterval:   10 * time.Millisecond,
		ErrorHandler: func(err error) error {
			return err // to force return after first request
		},
	}
	err := RunPushGatewayGatherer(ctx, config)

	assert.EqualError(t, err, "code=400, message=post metrics request did not succeed")
	assert.True(t, receivedMetrics)
}

func requestBody(e *echo.Echo, path string) (string, int) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	return rec.Body.String(), rec.Code
}

func request(e *echo.Echo, path string) int {
	_, code := requestBody(e, path)
	return code
}

func unregisterDefaults(subsystem string) {
	// this is extremely hacky way to unregister our middleware metrics that it registers to prometheus default registry
	// Metrics/collector can be unregistered only by their instance but we do not have their instance, so we need to
	// create similar collector to register it and get error back with that existing collector we actually want to
	// unregister
	p := prometheus.DefaultRegisterer

	unRegisterCollector := func(opts prometheus.Opts) {
		dummyDuplicate := prometheus.NewCounterVec(prometheus.CounterOpts(opts), []string{"code", "method", "host", "url"})
		err := p.Register(dummyDuplicate)
		if err == nil {
			return
		}
		var arErr prometheus.AlreadyRegisteredError
		if errors.As(err, &arErr) {
			p.Unregister(arErr.ExistingCollector)
		}
	}

	unRegisterCollector(prometheus.Opts{
		Subsystem: subsystem,
		Name:      "requests_total",
		Help:      "How many HTTP requests processed, partitioned by status code and HTTP method.",
	})
	unRegisterCollector(prometheus.Opts{
		Subsystem: subsystem,
		Name:      "request_duration_seconds",
		Help:      "The HTTP request latencies in seconds.",
	})
	unRegisterCollector(prometheus.Opts{
		Subsystem: subsystem,
		Name:      "response_size_bytes",
		Help:      "The HTTP response sizes in bytes.",
	})
	unRegisterCollector(prometheus.Opts{
		Subsystem: subsystem,
		Name:      "request_size_bytes",
		Help:      "The HTTP request sizes in bytes.",
	})
}
