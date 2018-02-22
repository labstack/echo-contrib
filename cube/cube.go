package cube

import (
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

		// LabStack account id
		AccountID string `json:"account_id"`

		// LabStack api key
		APIKey string `json:"api_key"`

		// API node
		Node string `json:"node"`

		// API group
		Group string `json:"group"`

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
		panic("echo: labstack analytics middleware requires an api key")
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
	cube.Node = config.Node
	cube.Group = config.Group
	cube.Tags = config.Tags
	cube.BatchSize = config.BatchSize
	cube.DispatchInterval = config.DispatchInterval
	cube.ClientLookup = config.ClientLookup

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			if config.Skipper(c) {
				return next(c)
			}

			// Start
			r := cube.Start(c.Request(), c.Response())

			// Next
			if err = next(c); err != nil {
				c.Error(err)
			}

			// Stop
			cube.Stop(r, c.Response().Status, c.Response().Size)

			return nil
		}
	}
}
