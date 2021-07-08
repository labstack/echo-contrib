package casbin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func testRequest(t *testing.T, h echo.HandlerFunc, user string, path string, method string, code int) {
	e := echo.New()
	req := httptest.NewRequest(method, path, nil)
	req.SetBasicAuth(user, "secret")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	err := h(c)

	if err != nil {
		if errObj, ok := err.(*echo.HTTPError); ok {
			if errObj.Code != code {
				t.Errorf("%s, %s, %s: %d, supposed to be %d", user, path, method, errObj.Code, code)
			}
		} else {
			t.Error(err)
		}
	} else {
		if c.Response().Status != code {
			t.Errorf("%s, %s, %s: %d, supposed to be %d", user, path, method, c.Response().Status, code)
		}
	}
}

func TestAuth(t *testing.T) {
	ce, _ := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	h := Middleware(ce)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	testRequest(t, h, "alice", "/dataset1/resource1", echo.GET, 200)
	testRequest(t, h, "alice", "/dataset1/resource1", echo.POST, 200)
	testRequest(t, h, "alice", "/dataset1/resource2", echo.GET, 200)
	testRequest(t, h, "alice", "/dataset1/resource2", echo.POST, 403)
}

func TestPathWildcard(t *testing.T) {
	ce, _ := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	h := Middleware(ce)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	testRequest(t, h, "bob", "/dataset2/resource1", "GET", 200)
	testRequest(t, h, "bob", "/dataset2/resource1", "POST", 200)
	testRequest(t, h, "bob", "/dataset2/resource1", "DELETE", 200)
	testRequest(t, h, "bob", "/dataset2/resource2", "GET", 200)
	testRequest(t, h, "bob", "/dataset2/resource2", "POST", 403)
	testRequest(t, h, "bob", "/dataset2/resource2", "DELETE", 403)

	testRequest(t, h, "bob", "/dataset2/folder1/item1", "GET", 403)
	testRequest(t, h, "bob", "/dataset2/folder1/item1", "POST", 200)
	testRequest(t, h, "bob", "/dataset2/folder1/item1", "DELETE", 403)
	testRequest(t, h, "bob", "/dataset2/folder1/item2", "GET", 403)
	testRequest(t, h, "bob", "/dataset2/folder1/item2", "POST", 200)
	testRequest(t, h, "bob", "/dataset2/folder1/item2", "DELETE", 403)
}

func TestRBAC(t *testing.T) {
	ce, _ := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	h := Middleware(ce)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	// cathy can access all /dataset1/* resources via all methods because it has the dataset1_admin role.
	testRequest(t, h, "cathy", "/dataset1/item", "GET", 200)
	testRequest(t, h, "cathy", "/dataset1/item", "POST", 200)
	testRequest(t, h, "cathy", "/dataset1/item", "DELETE", 200)
	testRequest(t, h, "cathy", "/dataset2/item", "GET", 403)
	testRequest(t, h, "cathy", "/dataset2/item", "POST", 403)
	testRequest(t, h, "cathy", "/dataset2/item", "DELETE", 403)

	// delete all roles on user cathy, so cathy cannot access any resources now.
	ce.DeleteRolesForUser("cathy")

	testRequest(t, h, "cathy", "/dataset1/item", "GET", 403)
	testRequest(t, h, "cathy", "/dataset1/item", "POST", 403)
	testRequest(t, h, "cathy", "/dataset1/item", "DELETE", 403)
	testRequest(t, h, "cathy", "/dataset2/item", "GET", 403)
	testRequest(t, h, "cathy", "/dataset2/item", "POST", 403)
	testRequest(t, h, "cathy", "/dataset2/item", "DELETE", 403)
}

func TestEnforceError(t *testing.T) {
	ce, _ := casbin.NewEnforcer("broken_auth_model.conf", "auth_policy.csv")
	h := Middleware(ce)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	testRequest(t, h, "cathy", "/dataset1/item", "GET", 500)
}

func TestCustomUserGetter(t *testing.T) {
	ce, _ := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	cnf := Config{
		Skipper:  middleware.DefaultSkipper,
		Enforcer: ce,
		UserGetter: func(c echo.Context) (string, error) {
			return "not_cathy_at_all", nil
		},
	}
	h := MiddlewareWithConfig(cnf)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})
	testRequest(t, h, "cathy", "/dataset1/item", "GET", 403)
}

func TestUserGetterError(t *testing.T) {
	ce, _ := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	cnf := Config{
		Skipper:  middleware.DefaultSkipper,
		Enforcer: ce,
		UserGetter: func(c echo.Context) (string, error) {
			return "", errors.New("no idea who you are")
		},
	}
	h := MiddlewareWithConfig(cnf)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})
	testRequest(t, h, "cathy", "/dataset1/item", "GET", 403)
}

func TestSyncedEnforcer(t *testing.T) {
	ce, _ := casbin.NewSyncedEnforcer("auth_model.conf", "auth_policy.csv")
	h := Middleware(ce)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	testRequest(t, h, "alice", "/dataset1/resource1", echo.GET, 200)
	testRequest(t, h, "alice", "/dataset1/resource1", echo.POST, 200)
	testRequest(t, h, "alice", "/dataset1/resource2", echo.GET, 200)
	testRequest(t, h, "alice", "/dataset1/resource2", echo.POST, 403)
}

func TestEnforcerNil(t *testing.T) {
	cnf := Config{
		Skipper: middleware.DefaultSkipper,
		UserGetter: func(c echo.Context) (string, error) {
			return "not_cathy_at_all", nil
		},
	}
	h := MiddlewareWithConfig(cnf)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	e := echo.New()
	req := httptest.NewRequest("GET", "/dataset1/item", nil)
	req.SetBasicAuth("cathy", "secret")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	if err := h(c); err == nil ||
		!strings.Contains(err.Error(), "casbin Enforcer is not set") {
		t.Error("expected the handler to fail with missing casbin Enforcer")
	}
}
