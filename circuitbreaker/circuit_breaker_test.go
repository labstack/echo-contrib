package circuitbreaker

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// TestNewCircuitBreaker ensures circuit breaker initializes with correct defaults
func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig)
	assert.Equal(t, StateClosed, cb.state)
	assert.Equal(t, DefaultCircuitBreakerConfig.Threshold, cb.threshold)
	assert.Equal(t, DefaultCircuitBreakerConfig.Timeout, cb.timeout)
	assert.Equal(t, DefaultCircuitBreakerConfig.SuccessReset, cb.successReset)
}

// TestAllowRequest checks request allowance in different states
func TestAllowRequest(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 3})

	assert.True(t, cb.AllowRequest())
	cb.ReportFailure()
	cb.ReportFailure()
	cb.ReportFailure()
	assert.False(t, cb.AllowRequest())
}

// TestReportSuccess verifies state transitions after successful requests
func TestReportSuccess(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 2, SuccessReset: 2})
	cb.state = StateHalfOpen
	cb.ReportSuccess()
	assert.Equal(t, StateHalfOpen, cb.state)
	cb.ReportSuccess()
	assert.Equal(t, StateClosed, cb.state)
}

// TestReportFailure checks state transitions after failures
func TestReportFailureThreshold(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 2})
	cb.ReportFailure()
	assert.Equal(t, StateClosed, cb.state)
	cb.ReportFailure()
	assert.Equal(t, StateOpen, cb.state)
}

// TestMiddlewareBlocksOpenState checks Middleware Blocks Requests in Open State
func TestMiddlewareBlocksOpenState(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig)
	cb.state = StateOpen // Force open state

	middleware := CircuitBreakerMiddleware(cb)(func(c echo.Context) error {
		return c.String(http.StatusOK, "Success")
	})

	err := middleware(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// TestHalfOpenLimitedRequests checks Half-Open state limits requests
func TestHalfOpenLimitedRequests(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig)
	cb.state = StateHalfOpen
	cb.halfOpenSemaphore <- struct{}{} // Simulate a request holding the slot

	assert.False(t, cb.AllowRequest()) // The next request should be blocked
}
