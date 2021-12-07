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
	"errors"
	"github.com/casbin/casbin/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
)

type (
	// Config defines the config for CasbinAuth middleware.
	Config struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// Enforcer CasbinAuth main rule.
		// One of Enforcer or EnforceHandler fields is required.
		Enforcer *casbin.Enforcer

		// EnforceHandler is custom callback to handle enforcing.
		// One of Enforcer or EnforceHandler fields is required.
		EnforceHandler func(c echo.Context, user string) (bool, error)

		// Method to get the username - defaults to using basic auth
		UserGetter func(c echo.Context) (string, error)

		// Method to handle errors
		ErrorHandler func(c echo.Context, internal error, proposedStatus int) error
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
		ErrorHandler: func(c echo.Context, internal error, proposedStatus int) error {
			err := echo.NewHTTPError(proposedStatus, internal.Error())
			err.Internal = internal
			return err
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
	if config.Enforcer == nil && config.EnforceHandler == nil {
		panic("one of casbin middleware Enforcer or EnforceHandler fields must be set")
	}
	if config.Skipper == nil {
		config.Skipper = DefaultConfig.Skipper
	}
	if config.UserGetter == nil {
		config.UserGetter = DefaultConfig.UserGetter
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = DefaultConfig.ErrorHandler
	}
	if config.EnforceHandler == nil {
		config.EnforceHandler = func(c echo.Context, user string) (bool, error) {
			return config.Enforcer.Enforce(user, c.Request().URL.Path, c.Request().Method)
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			user, err := config.UserGetter(c)
			if err != nil {
				return config.ErrorHandler(c, err, http.StatusForbidden)
			}
			pass, err := config.EnforceHandler(c, user)
			if err != nil {
				return config.ErrorHandler(c, err, http.StatusInternalServerError)
			}
			if !pass {
				return config.ErrorHandler(c, errors.New("enforce did not pass"), http.StatusForbidden)
			}
			return next(c)
		}
	}
}
