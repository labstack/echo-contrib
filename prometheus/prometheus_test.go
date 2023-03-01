package prometheus

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func unregister(p *Prometheus) {
	prometheus.Unregister(p.reqCnt)
	prometheus.Unregister(p.reqDur)
	prometheus.Unregister(p.reqSz)
	prometheus.Unregister(p.resSz)
}

func TestPrometheus_Use(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Use(e)

	assert.Equal(t, 1, len(e.Routes()), "only one route should be added")
	assert.NotNil(t, e, "the engine should not be empty")
	assert.Equal(t, e.Routes()[0].Path, p.MetricsPath, "the path should match the metrics path")
	unregister(p)
}

func TestPrometheus_Buckets(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Use(e)

	path := "/ping"

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	req = httptest.NewRequest(http.MethodGet, p.MetricsPath, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), fmt.Sprintf("%s_request_duration_seconds", p.Subsystem))

	body := rec.Body.String()
	assert.Contains(t, body, `echo_request_duration_seconds_bucket{code="404",host="example.com",method="GET",url="/ping",le="0.005"}`, "duration should have time bucket (like, 0.005s)")
	assert.NotContains(t, body, `echo_request_duration_seconds_bucket{code="404",host="example.com",method="GET",url="/ping",le="512000"}`, "duration should NOT have a size bucket (like, 512K)")
	assert.Contains(t, body, `echo_request_size_bytes_bucket{code="404",host="example.com",method="GET",url="/ping",le="1024"}`, "request size should have a 1024k (size) bucket")
	assert.NotContains(t, body, `echo_request_size_bytes_bucket{code="404",host="example.com",method="GET",url="/ping",le="0.005"}`, "request size should NOT have time bucket (like, 0.005s)")
	assert.Contains(t, body, `echo_response_size_bytes_bucket{code="404",host="example.com",method="GET",url="/ping",le="1024"}`, "response size should have a 1024k (size) bucket")
	assert.NotContains(t, body, `echo_response_size_bytes_bucket{code="404",host="example.com",method="GET",url="/ping",le="0.005"}`, "response size should NOT have time bucket (like, 0.005s)")

	unregister(p)
}

func TestPath(t *testing.T) {
	p := NewPrometheus("echo", nil)
	assert.Equal(t, p.MetricsPath, defaultMetricPath, "no usage of path should yield default path")
	unregister(p)
}

func TestSubsystem(t *testing.T) {
	p := NewPrometheus("echo", nil)
	assert.Equal(t, p.Subsystem, "echo", "subsystem should be default")
	unregister(p)
}

func TestUse(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)

	req := httptest.NewRequest(http.MethodGet, p.MetricsPath, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	p.Use(e)

	req = httptest.NewRequest(http.MethodGet, p.MetricsPath, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	unregister(p)
}

func TestIgnore(t *testing.T) {
	e := echo.New()

	ipath := "/ping"
	lipath := fmt.Sprintf(`path="%s"`, ipath)
	ignore := func(c echo.Context) bool {
		if strings.HasPrefix(c.Path(), ipath) {
			return true
		}
		return false
	}
	p := NewPrometheus("echo", ignore)
	p.Use(e)

	req := httptest.NewRequest(http.MethodGet, p.MetricsPath, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), fmt.Sprintf("%s_requests_total", p.Subsystem))

	req = httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	req = httptest.NewRequest(http.MethodGet, p.MetricsPath, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), lipath, "ignored path must not be present")

	unregister(p)
}

func TestMetricsGenerated(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Use(e)

	req := httptest.NewRequest(http.MethodGet, "/ping?test=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	req = httptest.NewRequest(http.MethodGet, p.MetricsPath, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	s := rec.Body.String()
	assert.Contains(t, s, `url="/ping"`, "path must be present")
	assert.Contains(t, s, `host="example.com"`, "host must be present")

	unregister(p)
}

func TestMetricsPathIgnored(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Use(e)

	req := httptest.NewRequest(http.MethodGet, p.MetricsPath, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), fmt.Sprintf("%s_requests_total", p.Subsystem))
	unregister(p)
}

func TestMetricsPushGateway(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Use(e)

	req := httptest.NewRequest(http.MethodGet, p.MetricsPath, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), fmt.Sprintf("%s_request_duration", p.Subsystem))

	unregister(p)
}

func TestMetricsForErrors(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Use(e)

	e.GET("/handler_for_ok", func(c echo.Context) error {
		return c.JSON(http.StatusOK, "OK")
	})
	e.GET("/handler_for_nok", func(c echo.Context) error {
		return c.JSON(http.StatusConflict, "NOK")
	})
	e.GET("/handler_for_error", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusBadGateway, "BAD")
	})

	req := httptest.NewRequest(http.MethodGet, "/handler_for_ok", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/handler_for_nok", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusConflict, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/handler_for_nok", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusConflict, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/handler_for_error", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadGateway, rec.Code)

	req = httptest.NewRequest(http.MethodGet, p.MetricsPath, nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, fmt.Sprintf("%s_requests_total", p.Subsystem))
	assert.Contains(t, body, `echo_requests_total{code="200",host="example.com",method="GET",url="/handler_for_ok"} 1`)
	assert.Contains(t, body, `echo_requests_total{code="409",host="example.com",method="GET",url="/handler_for_nok"} 2`)
	assert.Contains(t, body, `echo_requests_total{code="502",host="example.com",method="GET",url="/handler_for_error"} 1`)

	unregister(p)
}
