package pprof

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestPProfRegisterDefaualtPrefix(t *testing.T) {
	var pprofPaths = []struct {
		path string
	}{
		{"/"},
		{"/allocs"},
		{"/block"},
		{"/cmdline"},
		{"/goroutine"},
		{"/heap"},
		{"/mutex"},
		{"/profile?seconds=1"},
		{"/symbol"},
		{"/symbol"},
		{"/threadcreate"},
		{"/trace"},
	}
	for _, tt := range pprofPaths {
		t.Run(tt.path, func(t *testing.T) {
			e := echo.New()
			Register(e)
			req, _ := http.NewRequest(http.MethodGet, DefaultPrefix+tt.path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, rec.Code, http.StatusOK)
		})
	}
}

func TestPProfRegisterCustomPrefix(t *testing.T) {
	var pprofPaths = []struct {
		path string
	}{
		{"/"},
		{"/allocs"},
		{"/block"},
		{"/cmdline"},
		{"/goroutine"},
		{"/heap"},
		{"/mutex"},
		{"/profile?seconds=1"},
		{"/symbol"},
		{"/symbol"},
		{"/threadcreate"},
		{"/trace"},
	}
	for _, tt := range pprofPaths {
		t.Run(tt.path, func(t *testing.T) {
			e := echo.New()
			pprofPrefix := "/myapp/pprof"
			Register(e, pprofPrefix)
			req, _ := http.NewRequest(http.MethodGet, pprofPrefix+tt.path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, rec.Code, http.StatusOK)
		})
	}
}
