package sqlcommenter

import (
	"fmt"

	"github.com/google/sqlcommenter/go/core"
	"github.com/google/sqlcommenter/go/net/http"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Config struct {
	// Skipper defines a function to skip middleware.
	Skipper middleware.Skipper
}

var DefaultConfig = Config{
	Skipper: middleware.DefaultSkipper,
}

func Middleware() echo.MiddlewareFunc {
	return MiddlewareWithConfig(DefaultConfig)
}

func MiddlewareWithConfig(config Config) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = DefaultConfig.Skipper
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			// Since echo wrapped the registered handler (see https://github.com/labstack/echo/blob/v4.10.2/echo.go#L573),
			// we can't get the original handler name, so we use empty action here. But that's not a big deal,
			// because we can locate the code by the route.
			route := fmt.Sprintf("%s %s", c.Request().Method, c.Path())
			tags := http.NewHTTPRequestTags("labstack/echo", route, "")
			ctx := core.ContextInject(c.Request().Context(), tags)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}
