package prometheus

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/appleboy/gofight/v2"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestPrometheus_Use(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Registerer = prometheus.NewRegistry()
	p.Use(e)

	assert.Equal(t, 1, len(e.Routes()), "only one route should be added")
	assert.NotNil(t, e, "the engine should not be empty")
	assert.Equal(t, e.Routes()[0].Path, p.MetricsPath, "the path should match the metrics path")
}

func TestPrometheus_Buckets(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Registerer = prometheus.NewRegistry()
	p.Use(e)

	path := "/ping"

	g := gofight.New()
	g.GET(path).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) { assert.Equal(t, http.StatusNotFound, r.Code) })

	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Contains(t, r.Body.String(), fmt.Sprintf("%s_request_duration_seconds", p.Subsystem))
		assert.Regexp(t, "request_duration_seconds.*le=\"0.005\"", r.Body.String(), "duration should have time bucket (like, 0.005s)")
		assert.NotRegexp(t, "request_duration_seconds.*le=\"512000\"", r.Body.String(), "duration should NOT have a size bucket (like, 512K)")
		assert.Regexp(t, "response_size_bytes.*le=\"512000\"", r.Body.String(), "response size should have a 512K (size) bucket")
		assert.NotRegexp(t, "response_size_bytes.*le=\"0.005\"", r.Body.String(), "response size should NOT have time bucket (like, 0.005s)")
		assert.Regexp(t, "request_size_bytes.*le=\"512000\"", r.Body.String(), "request size should have a 512K (size) bucket")
		assert.NotRegexp(t, "request_size_bytes.*le=\"0.005\"", r.Body.String(), "request should NOT have time bucket (like, 0.005s)")
	})

}

func TestPath(t *testing.T) {
	p := NewPrometheus("echo", nil)
	p.Registerer = prometheus.NewRegistry()
	assert.Equal(t, p.MetricsPath, defaultMetricPath, "no usage of path should yield default path")
}

func TestSubsystem(t *testing.T) {
	p := NewPrometheus("echo", nil)
	p.Registerer = prometheus.NewRegistry()
	assert.Equal(t, p.Subsystem, "echo", "subsystem should be default")
}

func TestUse(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Registerer = prometheus.NewRegistry()

	g := gofight.New()
	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusNotFound, r.Code)
	})

	p.Use(e)

	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusOK, r.Code)
	})
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
	p.Registerer = prometheus.NewRegistry()
	p.Use(e)

	g := gofight.New()

	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.NotContains(t, r.Body.String(), fmt.Sprintf("%s_requests_total", p.Subsystem))
	})

	g.GET("/ping").Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) { assert.Equal(t, http.StatusNotFound, r.Code) })

	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.NotContains(t, r.Body.String(), fmt.Sprintf("%s_requests_total", p.Subsystem))
		assert.NotContains(t, r.Body.String(), lipath, "ignored path must not be present")
	})
}

func TestMetricsGenerated(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Registerer = prometheus.NewRegistry()
	p.Use(e)

	path := "/ping"
	lpath := fmt.Sprintf(`url="%s"`, path)

	g := gofight.New()
	g.GET(path).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) { assert.Equal(t, http.StatusNotFound, r.Code) })

	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Contains(t, r.Body.String(), fmt.Sprintf("%s_requests_total", p.Subsystem))
		assert.Contains(t, r.Body.String(), lpath, "path must be present")
	})
}

func TestMetricsPathIgnored(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Registerer = prometheus.NewRegistry()
	p.Use(e)

	g := gofight.New()
	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.NotContains(t, r.Body.String(), fmt.Sprintf("%s_requests_total", p.Subsystem))
	})
}

func TestMetricsPushGateway(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Registerer = prometheus.NewRegistry()
	p.Use(e)

	g := gofight.New()
	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.NotContains(t, r.Body.String(), fmt.Sprintf("%s_request_duration", p.Subsystem))
	})
}

func TestMetricsForErrors(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Registerer = prometheus.NewRegistry()
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

	g := gofight.New()
	g.GET("/handler_for_ok").Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) { assert.Equal(t, http.StatusOK, r.Code) })

	g.GET("/handler_for_nok").Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) { assert.Equal(t, http.StatusConflict, r.Code) })
	g.GET("/handler_for_nok").Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) { assert.Equal(t, http.StatusConflict, r.Code) })

	g.GET("/handler_for_error").Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) { assert.Equal(t, http.StatusBadGateway, r.Code) })

	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusOK, r.Code)
		body := r.Body.String()
		assert.Contains(t, body, fmt.Sprintf("%s_requests_total", p.Subsystem))
		assert.Contains(t, body, `echo_requests_total{code="200",host="",method="GET",url="/handler_for_ok"} 1`)
		assert.Contains(t, body, `echo_requests_total{code="409",host="",method="GET",url="/handler_for_nok"} 2`)
		assert.Contains(t, body, `echo_requests_total{code="502",host="",method="GET",url="/handler_for_error"} 1`)
	})
}
