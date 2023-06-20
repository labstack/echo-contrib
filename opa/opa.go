/*
import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo-contrib/opaecho"
)

func main() {
	e := echo.New()

	module := `
		package example.authz
		default allow := false
		allow {
			input.Method == "GET"
		}
	`

	cfg := opaecho.Config{
		RegoQuery:             "data.example.authz.allow",
		RegoPolicy:            bytes.NewBufferString(module),
		IncludeQueryString:    true,
		DeniedStatusCode:      http.StatusForbidden,
		DeniedResponseMessage: "status forbidden",
		IncludeHeaders:        []string{"Authorization"},
	}

	e.Use(opaecho.MiddlewareWithConfig(cfg))

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	e.Logger.Fatal(e.Start(":8080"))
}
*/




package opaecho

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/open-policy-agent/opa/rego"
)

type (
	// Config defines the config for OPA middleware.
	Config struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// RegoPolicy OPA policy rule.
		RegoPolicy io.Reader

		// RegoQuery OPA policy rule.
		RegoQuery string

		// IncludeHeaders headers to include in the input.
		IncludeHeaders []string

		// IncludeQueryString whether to include the query string in the input.
		IncludeQueryString bool

		// DeniedStatusCode status code to return when the request is denied.
		DeniedStatusCode int

		// DeniedResponseMessage response message when the request is denied.
		DeniedResponseMessage string

		// InputCreationMethod Function to generate the input.
		InputCreationMethod InputCreationFunc

		// ErrorHandler is custom callback to handle enforcing.
		ErrorHandler func(c echo.Context, internal error, proposedStatus int) error
	}

	// InputCreationFunc is the function signature for functions that generate the input for the Rego query.
	InputCreationFunc func(c echo.Context) (map[string]interface{}, error)
)

var (
	// DefaultConfig is the default OPA middleware config.
	DefaultConfig = Config{
		
		RegoQuery: "data.example.authz.allow",
		Skipper: middleware.DefaultSkipper,
		DeniedStatusCode: http.StatusBadRequest,
		DeniedResponseMessage: "Request denied",
		InputCreationMethod: defaultInput,
		ErrorHandler: func(c echo.Context, internal error, proposedStatus int) error {
			err := echo.NewHTTPError(proposedStatus, internal.Error())
			err.Internal = internal
			return err
		},
	}
)

// MiddlewareWithConfig returns a OPA middleware with config.
// See `Middleware()`.
func MiddlewareWithConfig(config Config) echo.MiddlewareFunc {
	// fillAndValidate checks if any mandatory field is missing.
	if err := config.fillAndValidate(); err != nil {
		panic(err)
	}

	policy, err := ioutil.ReadAll(config.RegoPolicy)
	if err != nil {
		panic(fmt.Errorf("could not read rego policy: %w", err))
	}

	query, err := rego.New(
		rego.Query(config.RegoQuery),
		rego.Module("policy.rego", string(policy)),
	).PrepareForEval(context.Background())

	if err != nil {
		panic(fmt.Errorf("rego policy error: %w", err))
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			input, err := config.InputCreationMethod(c)
			if err != nil {
				return config.ErrorHandler(c, err, http.StatusInternalServerError)
			}

			if config.IncludeQueryString {
				input["query"] = c.QueryParams()
			}

			if len(config.IncludeHeaders) > 0 {
				headers := make(map[string]string)
				for _, header := range config.IncludeHeaders {
					headers[header] = c.Request().Header.Get(header)
				}
				input["headers"] = headers
			}

			res, err := query.Eval(context.Background(), rego.EvalInput(input))
			if err != nil {
				return config.ErrorHandler(c, err, http.StatusInternalServerError)
			}

			if !res.Allowed() {
				return config.ErrorHandler(c, errors.New("request denied"), config.DeniedStatusCode)
			}

			return next(c)
		}
	}
}

// fillAndValidate fills in default values for missing configuration fields and validates the configuration.
func (c *Config) fillAndValidate() error {
	if c.RegoQuery == "" {
		return errors.New("rego query can not be empty")
	}

	if c.DeniedStatusCode == 0 {
		c.DeniedStatusCode = DefaultConfig.DeniedStatusCode
	}

	if c.DeniedResponseMessage == "" {
		c.DeniedResponseMessage = DefaultConfig.DeniedResponseMessage
	}

	if c.Skipper == nil {
		c.Skipper = DefaultConfig.Skipper
	}

	if c.InputCreationMethod == nil {
		c.InputCreationMethod = DefaultConfig.InputCreationMethod
	}

	if c.ErrorHandler == nil {
		c.ErrorHandler = DefaultConfig.ErrorHandler
	}

	return nil
}

// defaultInput is the default input generation function.
func defaultInput(c echo.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"method": c.Request().Method,   // Include the HTTP method in the input.
		"path":   c.Request().URL.Path, // Include the request path in the input.
	}, nil
}