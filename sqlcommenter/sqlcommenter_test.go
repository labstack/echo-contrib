package sqlcommenter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/sqlcommenter/go/core"
	"github.com/labstack/echo/v4"
)

func TestMiddleware(t *testing.T) {
	framework := "labstack/echo"
	route := "GET /test/:id"
	action := ""

	handler := func(c echo.Context) error {
		ctx := c.Request().Context()
		_framework := ctx.Value(core.Framework)
		_route := ctx.Value(core.Route)
		_action := ctx.Value(core.Action)

		if _framework != framework {
			t.Errorf("mismatched framework - got: %s, want: %s", _framework, framework)
		}

		if _route != route {
			t.Errorf("mismatched route - got: %s, want: %s", _route, route)
		}

		if _action != action {
			t.Errorf("mismatched action - got: %s, want: %s", _action, action)
		}

		return nil
	}

	e := echo.New()
	e.Use(Middleware())
	e.GET("/test/:id", handler)

	rr := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/test/1", nil)

	if err != nil {
		t.Errorf("error while building req: %v", err)
	}

	e.ServeHTTP(rr, req)
}
