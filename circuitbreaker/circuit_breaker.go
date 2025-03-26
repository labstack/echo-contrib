// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2017 LabStack and Echo contributors

// Package circuitbreaker provides a circuit breaker middleware for Echo.
package circuitbreaker

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "closed"    // Normal operation
	StateOpen     CircuitBreakerState = "open"      // Requests are blocked
	StateHalfOpen CircuitBreakerState = "half-open" // Limited requests allowed to check recovery
)

// CircuitBreaker controls the flow of requests based on failure thresholds
type CircuitBreaker struct {
	failureCount int
	successCount int
	state        CircuitBreakerState
	mutex        sync.Mutex
	threshold    int
	timeout      time.Duration
	resetTimeout time.Duration
	successReset int
	lastFailure  time.Time
	exitChan     chan struct{}
}

// CircuitBreakerConfig holds configuration options for the circuit breaker
type CircuitBreakerConfig struct {
	Threshold    int                          // Maximum failures before switching to open state
	Timeout      time.Duration                // Time window before attempting recovery
	ResetTimeout time.Duration                // Interval for monitoring the circuit state
	SuccessReset int                          // Number of successful attempts to move to closed state
	OnOpen       func(ctx echo.Context) error // Callback for open state
	OnHalfOpen   func(ctx echo.Context) error // Callback for half-open state
	OnClose      func(ctx echo.Context) error // Callback for closed state
}

// Default configuration values for the circuit breaker
var DefaultCircuitBreakerConfig = CircuitBreakerConfig{
	Threshold:    5,
	Timeout:      30 * time.Second,
	ResetTimeout: 10 * time.Second,
	SuccessReset: 3,
	OnOpen: func(ctx echo.Context) error {
		return ctx.JSON(http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
	},
	OnHalfOpen: func(ctx echo.Context) error {
		return ctx.JSON(http.StatusTooManyRequests, map[string]string{"error": "service under recovery"})
	},
	OnClose: nil,
}

// NewCircuitBreaker initializes a circuit breaker with the given configuration
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.Threshold <= 0 {
		config.Threshold = DefaultCircuitBreakerConfig.Threshold
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultCircuitBreakerConfig.Timeout
	}
	if config.ResetTimeout == 0 {
		config.ResetTimeout = DefaultCircuitBreakerConfig.ResetTimeout
	}
	if config.SuccessReset <= 0 {
		config.SuccessReset = DefaultCircuitBreakerConfig.SuccessReset
	}
	if config.OnOpen == nil {
		config.OnOpen = DefaultCircuitBreakerConfig.OnOpen
	}
	if config.OnHalfOpen == nil {
		config.OnHalfOpen = DefaultCircuitBreakerConfig.OnHalfOpen
	}

	cb := &CircuitBreaker{
		threshold:    config.Threshold,
		timeout:      config.Timeout,
		resetTimeout: config.ResetTimeout,
		successReset: config.SuccessReset,
		state:        StateClosed,
		exitChan:     make(chan struct{}),
	}
	go cb.monitorReset()
	return cb
}

// monitorReset periodically checks if the circuit should move from open to half-open state
func (cb *CircuitBreaker) monitorReset() {
	for {
		select {
		case <-time.After(cb.resetTimeout):
			cb.mutex.Lock()
			if cb.state == StateOpen && time.Since(cb.lastFailure) > cb.timeout {
				cb.state = StateHalfOpen
				cb.successCount = 0
				cb.failureCount = 0 // Reset failure count
			}
			cb.mutex.Unlock()
		case <-cb.exitChan:
			return
		}
	}
}

// AllowRequest checks if requests are allowed based on the circuit state
func (cb *CircuitBreaker) AllowRequest() bool {

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	return cb.state != StateOpen
}

// ReportSuccess updates the circuit breaker on a successful request
func (cb *CircuitBreaker) ReportSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.successCount++
	if cb.state == StateHalfOpen && cb.successCount >= cb.successReset {
		cb.state = StateClosed
		cb.failureCount = 0
		cb.successCount = 0
	}
}

// ReportFailure updates the circuit breaker on a failed request
func (cb *CircuitBreaker) ReportFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount++
	cb.lastFailure = time.Now()

	if cb.failureCount >= cb.threshold {
		cb.state = StateOpen
	}
}

// CircuitBreakerMiddleware applies the circuit breaker to Echo requests
func CircuitBreakerMiddleware(config CircuitBreakerConfig) echo.MiddlewareFunc {
	cb := NewCircuitBreaker(config)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if !cb.AllowRequest() {
				return config.OnOpen(ctx)
			}

			err := next(ctx)
			if err != nil {
				cb.ReportFailure()
				return err
			}

			cb.ReportSuccess()
			return nil
		}
	}
}
