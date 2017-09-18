package reporter

import (
	"fmt"
	"runtime"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/labstack-go"
)

type (
	// Config defines the config for Reporter middleware.
	Config struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// App ID
		AppID string

		// App name
		AppName string

		// LabStack Account ID
		AccountID string `json:"account_id"`

		// LabStack API key
		APIKey string `json:"api_key"`

		// Headers to include
		Headers []string `json:"headers"`

		// TODO: To be implemented
		ClientLookup string `json:"client_lookup"`
	}
)

var (
	// DefaultConfig is the default Reporter middleware config.
	DefaultConfig = Config{
		Skipper: middleware.DefaultSkipper,
	}
)

// Middleware implements Reporter middleware.
func Middleware(accountID string, apiKey string) echo.MiddlewareFunc {
	c := DefaultConfig
	c.AccountID = accountID
	c.APIKey = apiKey
	return MiddlewareWithConfig(c)
}

// MiddlewareWithConfig returns a Reporter middleware with config.
// See: `Middleware()`.
func MiddlewareWithConfig(config Config) echo.MiddlewareFunc {
	// Defaults
	if config.APIKey == "" {
		panic("echo: reporter middleware requires an api key")
	}
	if config.Skipper == nil {
		config.Skipper = DefaultConfig.Skipper
	}

	// Initialize
	client := labstack.NewClient(config.AccountID, config.APIKey)
	log := client.Log()
	log.Fields.Add("app_id", config.AppID).
		Add("app_name", config.AppName)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			if config.Skipper(c) {
				return next(c)
			}

			// Capture error and non-fatal report
			c.Echo().HTTPErrorHandler = func(err error, c echo.Context) {
				c.Echo().DefaultHTTPErrorHandler(err, c)
				fields := labstack.Fields{}.
					Add("message", err.Error())
				appendFields(fields, c, config)
				log.Error(fields)
			}

			// Automatically report fatal error
			defer func() {
				if r := recover(); r != nil {
					var err error
					switch r := r.(type) {
					case error:
						err = r
					default:
						err = fmt.Errorf("%v", r)
					}
					stack := make([]byte, 4<<10) // 4 KB
					length := runtime.Stack(stack, false)
					fields := labstack.Fields{}.
						Add("message", err.Error()).
						Add("stack_trace", string(stack[:length]))
					appendFields(fields, c, config)
					log.Fatal(fields)
				}
			}()
			return next(c)
		}
	}
}

func appendFields(f labstack.Fields, c echo.Context, config Config) {
	f.
		Add("host", c.Request().Host).
		Add("path", c.Request().URL.Path).
		Add("method", c.Request().Method).
		Add("client_id", c.RealIP()).
		Add("remote_ip", c.RealIP()).
		Add("status", c.Response().Status)
	for _, h := range config.Headers {
		f.Add("header_"+h, c.Request().Header.Get(h))
	}
}
