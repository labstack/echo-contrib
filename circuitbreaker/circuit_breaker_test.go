package circuitbreaker

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// TestNewCircuitBreaker ensures circuit breaker initializes with correct defaults
func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{})
	assert.Equal(t, StateClosed, cb.state)
	assert.Equal(t, DefaultCircuitBreakerConfig.Threshold, cb.threshold)
	assert.Equal(t, DefaultCircuitBreakerConfig.Timeout, cb.timeout)
	assert.Equal(t, DefaultCircuitBreakerConfig.ResetTimeout, cb.resetTimeout)
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
func TestReportFailure(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 2})
	cb.ReportFailure()
	assert.Equal(t, StateClosed, cb.state)
	cb.ReportFailure()
	assert.Equal(t, StateOpen, cb.state)
}

// TestMonitorReset ensures circuit moves to half-open after timeout
func TestMonitorReset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 1, Timeout: 1 * time.Second, ResetTimeout: 500 * time.Millisecond})
	cb.ReportFailure()
	time.Sleep(2 * time.Second) // Wait for reset logic
	assert.Equal(t, StateHalfOpen, cb.state)
}

// TestCircuitBreakerMiddleware verifies middleware behavior
func TestCircuitBreakerMiddleware(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	handler := CircuitBreakerMiddleware(DefaultCircuitBreakerConfig)(func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	err := handler(ctx)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}
