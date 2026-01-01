// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2017 LabStack and Echo contributors

package casbin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/casbin/casbin/v2"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func testRequest(t *testing.T, h echo.HandlerFunc, user string, path string, method string, code int) {
	e := echo.New()
	req := httptest.NewRequest(method, path, nil)
	req.SetBasicAuth(user, "secret")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	err := h(c)

	if err != nil {
		var errObj *echo.HTTPError
		if errors.As(err, &errObj) {
			if errObj.Code != code {
				t.Errorf("%s, %s, %s: %d, supposed to be %d", user, path, method, errObj.Code, code)
			}
		}
	} else {
		status := 0
		if eResp, uErr := echo.UnwrapResponse(c.Response()); uErr == nil {
			status = eResp.Status
		}
		if status != code {
			t.Errorf("%s, %s, %s: %d, supposed to be %d", user, path, method, status, code)
		}
	}
}

func TestAuth(t *testing.T) {
	ce, _ := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	h := Middleware(ce)(func(c *echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	testRequest(t, h, "alice", "/dataset1/resource1", http.MethodGet, http.StatusOK)
	testRequest(t, h, "alice", "/dataset1/resource1", http.MethodPost, http.StatusOK)
	testRequest(t, h, "alice", "/dataset1/resource2", http.MethodGet, http.StatusOK)
	testRequest(t, h, "alice", "/dataset1/resource2", http.MethodPost, http.StatusForbidden)
}

func TestPathWildcard(t *testing.T) {
	ce, _ := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	h := Middleware(ce)(func(c *echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	testRequest(t, h, "bob", "/dataset2/resource1", http.MethodGet, http.StatusOK)
	testRequest(t, h, "bob", "/dataset2/resource1", http.MethodPost, http.StatusOK)
	testRequest(t, h, "bob", "/dataset2/resource1", http.MethodDelete, http.StatusOK)
	testRequest(t, h, "bob", "/dataset2/resource2", http.MethodGet, http.StatusOK)
	testRequest(t, h, "bob", "/dataset2/resource2", http.MethodPost, http.StatusForbidden)
	testRequest(t, h, "bob", "/dataset2/resource2", http.MethodDelete, http.StatusForbidden)

	testRequest(t, h, "bob", "/dataset2/folder1/item1", http.MethodGet, http.StatusForbidden)
	testRequest(t, h, "bob", "/dataset2/folder1/item1", http.MethodPost, http.StatusOK)
	testRequest(t, h, "bob", "/dataset2/folder1/item1", http.MethodDelete, http.StatusForbidden)
	testRequest(t, h, "bob", "/dataset2/folder1/item2", http.MethodGet, http.StatusForbidden)
	testRequest(t, h, "bob", "/dataset2/folder1/item2", http.MethodPost, http.StatusOK)
	testRequest(t, h, "bob", "/dataset2/folder1/item2", http.MethodDelete, http.StatusForbidden)
}

func TestRBAC(t *testing.T) {
	ce, _ := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	h := Middleware(ce)(func(c *echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	// cathy can access all /dataset1/* resources via all methods because it has the dataset1_admin role.
	testRequest(t, h, "cathy", "/dataset1/item", http.MethodGet, http.StatusOK)
	testRequest(t, h, "cathy", "/dataset1/item", http.MethodPost, http.StatusOK)
	testRequest(t, h, "cathy", "/dataset1/item", http.MethodDelete, http.StatusOK)
	testRequest(t, h, "cathy", "/dataset2/item", http.MethodGet, http.StatusForbidden)
	testRequest(t, h, "cathy", "/dataset2/item", http.MethodPost, http.StatusForbidden)
	testRequest(t, h, "cathy", "/dataset2/item", http.MethodDelete, http.StatusForbidden)

	// delete all roles on user cathy, so cathy cannot access any resources now.
	ce.DeleteRolesForUser("cathy")

	testRequest(t, h, "cathy", "/dataset1/item", http.MethodGet, http.StatusForbidden)
	testRequest(t, h, "cathy", "/dataset1/item", http.MethodPost, http.StatusForbidden)
	testRequest(t, h, "cathy", "/dataset1/item", http.MethodDelete, http.StatusForbidden)
	testRequest(t, h, "cathy", "/dataset2/item", http.MethodGet, http.StatusForbidden)
	testRequest(t, h, "cathy", "/dataset2/item", http.MethodPost, http.StatusForbidden)
	testRequest(t, h, "cathy", "/dataset2/item", http.MethodDelete, http.StatusForbidden)
}

func TestEnforceError(t *testing.T) {
	ce, _ := casbin.NewEnforcer("broken_auth_model.conf", "auth_policy.csv")
	h := Middleware(ce)(func(c *echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	testRequest(t, h, "cathy", "/dataset1/item", http.MethodGet, http.StatusInternalServerError)
}

func TestCustomUserGetter(t *testing.T) {
	ce, _ := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	cnf := Config{
		Skipper:  middleware.DefaultSkipper,
		Enforcer: ce,
		UserGetter: func(c *echo.Context) (string, error) {
			return "not_cathy_at_all", nil
		},
	}
	h := MiddlewareWithConfig(cnf)(func(c *echo.Context) error {
		return c.String(http.StatusOK, "test")
	})
	testRequest(t, h, "cathy", "/dataset1/item", http.MethodGet, http.StatusForbidden)
}

func TestUserGetterError(t *testing.T) {
	ce, _ := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	cnf := Config{
		Skipper:  middleware.DefaultSkipper,
		Enforcer: ce,
		UserGetter: func(c *echo.Context) (string, error) {
			return "", errors.New("no idea who you are")
		},
	}
	h := MiddlewareWithConfig(cnf)(func(c *echo.Context) error {
		return c.String(http.StatusOK, "test")
	})
	testRequest(t, h, "cathy", "/dataset1/item", http.MethodGet, http.StatusForbidden)
}

func TestCustomEnforceHandler(t *testing.T) {
	ce, err := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	assert.NoError(t, err)

	_, err = ce.AddPolicy("bob", "/user/bob", "PATCH_SELF")
	assert.NoError(t, err)

	cnf := Config{
		EnforceHandler: func(c *echo.Context, user string) (bool, error) {
			method := c.Request().Method
			if strings.HasPrefix(c.Request().URL.Path, "/user/bob") {
				method += "_SELF"
			}
			return ce.Enforce(user, c.Request().URL.Path, method)
		},
	}
	h := MiddlewareWithConfig(cnf)(func(c *echo.Context) error {
		return c.String(http.StatusOK, "test")
	})
	testRequest(t, h, "bob", "/dataset2/resource1", http.MethodGet, http.StatusOK)
	testRequest(t, h, "bob", "/user/alice", http.MethodPatch, http.StatusForbidden)
	testRequest(t, h, "bob", "/user/bob", http.MethodPatch, http.StatusOK)
}

func TestCustomSkipper(t *testing.T) {
	ce, _ := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	cnf := Config{
		Skipper: func(c *echo.Context) bool {
			return c.Request().URL.Path == "/dataset1/resource1"
		},
		Enforcer: ce,
	}
	h := MiddlewareWithConfig(cnf)(func(c *echo.Context) error {
		return c.String(http.StatusOK, "test")
	})
	testRequest(t, h, "alice", "/dataset1/resource1", http.MethodGet, http.StatusOK)
	testRequest(t, h, "alice", "/dataset1/resource2", http.MethodPost, http.StatusForbidden)
}
