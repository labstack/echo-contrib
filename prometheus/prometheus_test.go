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

	g := gofight.New()
	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusNotFound, r.Code)
	})

	p.Use(e)

	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusOK, r.Code)
	})
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
	unregister(p)
}

func TestMetricsGenerated(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
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
	unregister(p)
}

func TestMetricsPathIgnored(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Use(e)

	g := gofight.New()
	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.NotContains(t, r.Body.String(), fmt.Sprintf("%s_requests_total", p.Subsystem))
	})
	unregister(p)
}

func TestMetricsPushGateway(t *testing.T) {
	e := echo.New()
	p := NewPrometheus("echo", nil)
	p.Use(e)

	g := gofight.New()
	g.GET(p.MetricsPath).Run(e, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.NotContains(t, r.Body.String(), fmt.Sprintf("%s_request_duration", p.Subsystem))
	})
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
	unregister(p)
}
