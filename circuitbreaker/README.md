# Echo Circuit Breaker Middleware

A robust circuit breaker implementation for the [Echo](https://echo.labstack.com/) web framework, providing fault tolerance and graceful service degradation.

## Overview

The Circuit Breaker pattern helps prevent cascading failures in distributed systems. When dependencies fail or become slow, the circuit breaker "trips" and fails fast, preventing system overload and allowing time for recovery.

This middleware implements a full-featured circuit breaker with three states:

- **Closed** (normal operation): Requests flow through normally
- **Open** (failure mode): Requests are rejected immediately without reaching the protected service
- **Half-Open** (recovery testing): Limited requests are allowed through to test if the service has recovered

## Features

- Configurable failure and success thresholds
- Automatic state transitions based on error rates
- Customizable timeout periods
- Controlled recovery with half-open state limiting concurrent requests
- Support for custom failure detection
- State transition callbacks
- Comprehensive metrics and monitoring

## Installation

```bash
go get github.com/labstack/echo-contrib/circuitbreaker
```

### Basic Usage

```go
package main

import (
    "github.com/labstack/echo/v4"
    "github.com/labstack/echo-contrib//circuitbreaker"
)

func main() {
    e := echo.New()
    
    // Create a circuit breaker with default configuration
    cb := circuitbreaker.New(circuitbreaker.DefaultConfig)
    
    // Apply it to specific routes
    e.GET("/protected", protectedHandler, circuitbreaker.Middleware(cb))
    
    e.Start(":8080")
}

func protectedHandler(c echo.Context) error {
    // Your handler code here
    return c.String(200, "Service is healthy")
}
```

### Advanced Usage

```go
cb := circuitbreaker.New(circuitbreaker.Config{
    FailureThreshold:      10,            // Number of failures before circuit opens
    Timeout:               30 * time.Second, // How long circuit stays open
    SuccessThreshold:      2,             // Successes needed to close circuit
    HalfOpenMaxConcurrent: 5,             // Max concurrent requests in half-open state
    IsFailure: func(c echo.Context, err error) bool {
        // Custom failure detection logic
        return err != nil || c.Response().Status >= 500
    },
    OnOpen: func(c echo.Context) error {
        // Custom handling when circuit opens
        return c.JSON(503, map[string]string{"status": "service temporarily unavailable"})
    },
})
```

### Monitoring and Metrics

```go
// Get current metrics
metrics := cb.Metrics()

// Add a metrics endpoint
e.GET("/metrics/circuit", func(c echo.Context) error {
    return c.JSON(200, cb.GetStateStats())
})
```

### State Management

```go
// Force circuit open (for maintenance, etc.)
cb.ForceOpen()

// Force circuit closed
cb.ForceClose()

// Reset circuit to initial state
cb.Reset()
```

### Best Practices

1. Use circuit breakers for external dependencies: APIs, databases, etc.
2. Set appropriate thresholds: Too low may cause premature circuit opening, too high may not protect effectively
3. Monitor circuit state: Add logging/metrics for state transitions
4. Consider service degradation: Provide fallbacks when circuit is open
5. Set timeouts appropriately: Match timeout to expected recovery time
