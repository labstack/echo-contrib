package cube

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/labstack/labstack-go"
)

type (
	// Config defines the config for Cube middleware.
	Config struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// LabStack Account ID
		AccountID string `json:"account_id"`

		// LabStack API key
		APIKey string `json:"api_key"`

		// Number of requests in a batch
		BatchSize int `json:"batch_size"`

		// Interval in seconds to dispatch the batch
		DispatchInterval time.Duration `json:"dispatch_interval"`

		// Additional tags
		Tags []string `json:"tags"`

		// TODO: To be implemented
		ClientLookup string `json:"client_lookup"`
	}
)

var (
	// DefaultConfig is the default Cube middleware config.
	DefaultConfig = Config{
		Skipper:          middleware.DefaultSkipper,
		BatchSize:        60,
		DispatchInterval: 60,
	}
)

// Middleware implements Cube middleware.
func Middleware(accountID, apiKey string) echo.MiddlewareFunc {
	c := DefaultConfig
	c.AccountID = accountID
	c.APIKey = apiKey
	return MiddlewareWithConfig(c)
}

// MiddlewareWithConfig returns a Cube middleware with config.
// See: `Middleware()`.
func MiddlewareWithConfig(config Config) echo.MiddlewareFunc {
	// Defaults
	if config.APIKey == "" {
		panic("echo: cube middleware requires an api key")
	}
	if config.Skipper == nil {
		config.Skipper = DefaultConfig.Skipper
	}
	if config.BatchSize == 0 {
		config.BatchSize = DefaultConfig.BatchSize
	}
	if config.DispatchInterval == 0 {
		config.DispatchInterval = DefaultConfig.DispatchInterval
	}

	// Initialize
	client := labstack.NewClient(config.AccountID, config.APIKey)
	cube := client.Cube()
	cube.APIKey = config.APIKey
	cube.BatchSize = config.BatchSize
	cube.DispatchInterval = config.DispatchInterval
	cube.ClientLookup = config.ClientLookup

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			if config.Skipper(c) {
				return next(c)
			}

			// Start
			cr := cube.Start(c.Request(), c.Response())

			// Handle panic
			defer func() {
				if r := recover(); r != nil {
					switch r := r.(type) {
					case error:
						err = r
					default:
						err = fmt.Errorf("%v", r)
					}
					stack := make([]byte, 4<<10) // 4 KB
					length := runtime.Stack(stack, false)
					cr.Error = err.Error()
					cr.StackTrace = string(stack[:length])
					println(c.Response().Status)
					if c.Response().Status == http.StatusOK {
						c.Response().Status = http.StatusInternalServerError
					}
				}

				// Stop
				cube.Stop(cr, c.Response().Status, c.Response().Size)
			}()

			// Next
			if err = next(c); err != nil {
				c.Error(err)
			}

			return nil
		}
	}
}
