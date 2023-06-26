package opaecho

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)


func TestMiddlewareWithConfig(t *testing.T) {
	e := echo.New()

	module := `
	package example.authz
	import future.keywords
	default allow := false
	`

	cfg := Config{
		RegoQuery:             "data.example.authz.allow",
		RegoPolicy:            bytes.NewBufferString(module),
		DeniedStatusCode:      http.StatusBadRequest,
		DeniedResponseMessage: "request denied",
	}
	e.Use(MiddlewareWithConfig(cfg))

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK!")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var responseBody map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &responseBody)
	assert.NoError(t, err)

	assert.Equal(t, "request denied", responseBody["message"])
}

func TestOpaRequestHeadersRegoPolicyShouldReturnOK(t *testing.T) {
	e := echo.New()

	module := `
	package example.authz
	import data.http

	default allow = true

	allow {
		req := input.request
		req.headers["testHeaderKey"] == "testHeaderVal"
	}
	`

	cfg := Config{
		RegoQuery:             "data.example.authz.allow",
		RegoPolicy:            bytes.NewBufferString(module),
		DeniedStatusCode:      http.StatusBadRequest,
		DeniedResponseMessage: "bad request",
		IncludeHeaders:        []string{"testHeaderKey"},
	}

	e.Use(MiddlewareWithConfig(cfg))

	e.GET("/headers", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/headers", nil)
	req.Header.Set("testHeaderKey", "testHeaderVal")

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}


