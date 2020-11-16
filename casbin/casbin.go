/* Package casbin provides middleware to enable ACL, RBAC, ABAC authorization support.

Simple example:

	package main

	import (
		"github.com/casbin/casbin/v2"
		"github.com/labstack/echo/v4"
		casbin_mw "github.com/labstack/echo-contrib/casbin"
	)

	func main() {
		e := echo.New()

		// Mediate the access for every request
		e.Use(casbin_mw.Middleware(casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")))

		e.Logger.Fatal(e.Start(":1323"))
	}

Advanced example:

	package main

	import (
		"github.com/casbin/casbin/v2"
		"github.com/labstack/echo/v4"
		casbin_mw "github.com/labstack/echo-contrib/casbin"
	)

	func main() {
		ce, _ := casbin.NewEnforcer("auth_model.conf", "")
		ce.AddRoleForUser("alice", "admin")
		ce.AddPolicy(...)

		e := echo.New()

		e.Use(casbin_mw.Middleware(ce))

		e.Logger.Fatal(e.Start(":1323"))
	}
*/

package casbin

import (
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type (
	// Config defines the config for CasbinAuth middleware.
	Config struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// Enforcer CasbinAuth main rule.
		// Required.
		Enforcer *casbin.Enforcer

		// Method to get the username - defaults to using basic auth
		UserGetter func(c echo.Context) (string, error)
	}
)

var (
	// DefaultConfig is the default CasbinAuth middleware config.
	DefaultConfig = Config{
		Skipper: middleware.DefaultSkipper,
		UserGetter: func(c echo.Context) (string, error) {
			username, _, _ := c.Request().BasicAuth()
			return username, nil
		},
	}
)

// Middleware returns a CasbinAuth middleware.
//
// For valid credentials it calls the next handler.
// For missing or invalid credentials, it sends "401 - Unauthorized" response.
func Middleware(ce *casbin.Enforcer) echo.MiddlewareFunc {
	c := DefaultConfig
	c.Enforcer = ce
	return MiddlewareWithConfig(c)
}

// MiddlewareWithConfig returns a CasbinAuth middleware with config.
// See `Middleware()`.
func MiddlewareWithConfig(config Config) echo.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultConfig.Skipper
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			if pass, err := config.CheckPermission(c); err == nil && pass {
				return next(c)
			} else if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}

			return echo.ErrForbidden
		}
	}
}

// GetUserName gets the user name from the request.
// Currently, only HTTP basic authentication is supported
func (a *Config) GetUserName(c echo.Context) string {
	username, _ := a.UserGetter(c)
	return username
}

// CheckPermission checks the user/method/path combination from the request.
// Returns true (permission granted) or false (permission forbidden)
func (a *Config) CheckPermission(c echo.Context) (bool, error) {
	user := a.GetUserName(c)
	method := c.Request().Method
	path := c.Request().URL.Path
	return a.Enforcer.Enforce(user, path, method)
}
