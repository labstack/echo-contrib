# Circuit Breaker Middleware for Echo

This package provides a custom Circuit Breaker middleware for the Echo framework in Golang. It helps protect your application from cascading failures by limiting requests to failing services and resetting based on configurable timeouts and success criteria.

## Features

- Configurable failure handling
- Timeout-based state reset
- Automatic transition between states: Closed, Open, and Half-Open
- Easy integration with Echo framework

## Usage

```go
package main

import (
	"net/http"
	"time"

	"github.com/labstack/echo-contrib/circuitbreaker"

	"github.com/labstack/echo/v4"
)

func main() {

	e := echo.New()

	cbConfig := circuitbreaker.Config{
		FailureThreshold: 5,                // Number of failures before opening circuit
		Timeout:          10 * time.Second, // Time to stay open before transitioning to half-open
		SuccessThreshold: 3,                // Number of successes needed to move back to closed state
	}

	cbMiddleware := circuitbreaker.New(cbConfig)

	e.GET("/example", func(c echo.Context) error {
		return c.String(http.StatusOK, "Success")
	}, circuitbreaker.Middleware(cbMiddleware))

	// Start server
	e.Logger.Fatal(e.Start(":8081"))
}
```

### Circuit Breaker States

1. **Closed**: Requests pass through normally. If failures exceed the threshold, it transitions to Open.
2. **Open**: Requests are blocked. After the timeout period, it moves to Half-Open.
3. **Half-Open**: Allows a limited number of test requests. If successful, it resets to Closed, otherwise, it goes back to Open.
