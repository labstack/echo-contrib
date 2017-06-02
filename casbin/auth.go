/* Package casbin provides middleware to enable ACL, RBAC, ABAC authorization support.

Simple example:

	package main

	import (
		"github.com/casbin/casbin"
		"github.com/labstack/echo"
		"github.com/labstack/echo-contrib/casbin" casbin-mw
	)

	func main() {
		e := echo.New()

		// Mediate the access for every request
		e.Use(casbin-mw.Auth(casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")))

		e.Logger.Fatal(e.Start(":1323"))
	}

Advanced example:

	package main

	import (
		"github.com/casbin/casbin"
		"github.com/labstack/echo"
		"github.com/labstack/echo-contrib/casbin" casbin-mw
	)

	func main() {
		ce := casbin.NewEnforcer("auth_model.conf", "")
		ce.AddRoleForUser("alice", "admin")
		ce.AddPolicy(...)

		e := echo.New()

		echo.Use(casbin-mw.Auth(ce))

		e.Logger.Fatal(e.Start(":1323"))
	}
*/

package casbin

import (
	"github.com/casbin/casbin"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

type (
	// AuthConfig defines the config for CasbinAuth middleware.
	AuthConfig struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper
		// Enforcer CasbinAuth main rule.
		// Required.
		Enforcer *casbin.Enforcer
	}
)

var (
	// DefaultAuthConfig is the default CasbinAuth middleware config.
	DefaultAuthConfig = AuthConfig{
		Skipper: middleware.DefaultSkipper,
	}
)

// Auth returns an Auth middleware.
//
// For valid credentials it calls the next handler.
// For missing or invalid credentials, it sends "401 - Unauthorized" response.
func Auth(ce *casbin.Enforcer) echo.MiddlewareFunc {
	c := DefaultAuthConfig
	c.Enforcer = ce
	return AuthWithConfig(c)
}

// AuthWithConfig returns a CasbinAuth middleware with config.
// See `Auth()`.
func AuthWithConfig(config AuthConfig) echo.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultAuthConfig.Skipper
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) || config.CheckPermission(c) {
				return next(c)
			}

			return echo.ErrForbidden
		}
	}
}

// GetUserName gets the user name from the request.
// Currently, only HTTP basic authentication is supported
func (a *AuthConfig) GetUserName(c echo.Context) string {
	username, _, _ := c.Request().BasicAuth()
	return username
}

// CheckPermission checks the user/method/path combination from the request.
// Returns true (permission granted) or false (permission forbidden)
func (a *AuthConfig) CheckPermission(c echo.Context) bool {
	user := a.GetUserName(c)
	method := c.Request().Method
	path := c.Request().URL.Path
	return a.Enforcer.Enforce(user, path, method)
}
