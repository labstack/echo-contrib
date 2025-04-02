package circuitbreaker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// TestNew ensures circuit breaker initializes with correct defaults
func TestNew(t *testing.T) {
	cb := New(DefaultConfig)
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, DefaultConfig.FailureThreshold, cb.failureThreshold)
	assert.Equal(t, DefaultConfig.Timeout, cb.timeout)
	assert.Equal(t, DefaultConfig.SuccessThreshold, cb.successThreshold)
}

// TestAllowRequest checks request allowance in different states
func TestAllowRequest(t *testing.T) {
	cb := New(Config{FailureThreshold: 3})

	allowed, _ := cb.AllowRequest()
	assert.True(t, allowed)

	cb.ReportFailure()
	cb.ReportFailure()
	cb.ReportFailure() // This should open the circuit

	allowed, _ = cb.AllowRequest()
	assert.False(t, allowed)
}

// TestReportSuccess verifies state transitions after successful requests
func TestReportSuccess(t *testing.T) {
	cb := New(Config{
		FailureThreshold: 2,
		SuccessThreshold: 2,
	})

	// Manually set to half-open state
	cb.state = StateHalfOpen

	cb.ReportSuccess()
	assert.Equal(t, StateHalfOpen, cb.GetState())

	cb.ReportSuccess()
	assert.Equal(t, StateClosed, cb.GetState())
}

// TestReportFailure checks state transitions after failures
func TestReportFailureThreshold(t *testing.T) {
	cb := New(Config{FailureThreshold: 2})

	cb.ReportFailure()
	assert.Equal(t, StateClosed, cb.GetState())

	cb.ReportFailure()
	assert.Equal(t, StateOpen, cb.GetState())
}

// TestMiddlewareBlocksOpenState checks Middleware Blocks Requests in Open State
func TestMiddlewareBlocksOpenState(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	cb := New(DefaultConfig)
	cb.ForceOpen() // Force open state

	middleware := Middleware(cb)(func(c echo.Context) error {
		return c.String(http.StatusOK, "Success")
	})

	err := middleware(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// TestHalfOpenLimitedRequests checks Half-Open state limits requests
func TestHalfOpenLimitedRequests(t *testing.T) {
	cb := New(Config{
		HalfOpenMaxConcurrent: 1,
	})

	// Manually set state to half-open
	cb.mutex.Lock()
	cb.state = StateHalfOpen
	cb.mutex.Unlock()

	// Take the only available slot
	cb.halfOpenSemaphore <- struct{}{}

	allowed, _ := cb.AllowRequest()
	assert.False(t, allowed, "Additional requests should be blocked in half-open state when all slots are taken")
}

// TestForceOpen tests the force open functionality
func TestForceOpen(t *testing.T) {
	cb := New(DefaultConfig)
	assert.Equal(t, StateClosed, cb.GetState())

	cb.ForceOpen()
	assert.Equal(t, StateOpen, cb.GetState())
}

// TestForceClose tests the force close functionality
func TestForceClose(t *testing.T) {
	cb := New(DefaultConfig)
	cb.ForceOpen()
	assert.Equal(t, StateOpen, cb.GetState())

	cb.ForceClose()
	assert.Equal(t, StateClosed, cb.GetState())
}

// TestStateTransitions tests full lifecycle transitions
func TestStateTransitions(t *testing.T) {
	// Create circuit breaker with short timeout for testing
	cb := New(Config{
		FailureThreshold: 2,
		Timeout:          50 * time.Millisecond,
		SuccessThreshold: 1,
	})

	// Initially should be closed
	assert.Equal(t, StateClosed, cb.GetState())

	// Report failures to trip the circuit
	cb.ReportFailure()
	cb.ReportFailure()

	// Should be open now
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for timeout to transition to half-open
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Report success to close the circuit
	cb.ReportSuccess()
	assert.Equal(t, StateClosed, cb.GetState())
}

// TestIsFailureFunction tests custom failure detection
func TestIsFailureFunction(t *testing.T) {
	customFailureCheck := func(c echo.Context, err error) bool {
		// Only consider 500+ errors as failures
		return err != nil || c.Response().Status >= 500
	}

	cb := New(Config{
		FailureThreshold: 2,
		IsFailure:        customFailureCheck,
	})

	e := echo.New()

	// Test with 400 status (should not be a failure)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// We need to actually set the status on the response writer
	c.Response().Status = http.StatusBadRequest

	// Should not count as failure
	assert.False(t, cb.config.IsFailure(c, nil))

	// Test with 500 status (should be a failure)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	// We need to actually set the status on the response writer
	c.Response().Status = http.StatusInternalServerError

	// Should count as failure
	assert.True(t, cb.config.IsFailure(c, nil))
}

// TestMiddlewareFullCycle tests middleware through a full request cycle
func TestMiddlewareFullCycle(t *testing.T) {
	e := echo.New()
	cb := New(Config{
		FailureThreshold: 2,
	})

	// Create a handler that fails
	failingHandler := Middleware(cb)(func(c echo.Context) error {
		return c.NoContent(http.StatusInternalServerError)
	})

	// Make two requests to trip the circuit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		_ = failingHandler(c)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	}

	// Circuit should be open now
	assert.Equal(t, StateOpen, cb.GetState())

	// Next request should be blocked by the circuit breaker
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = failingHandler(c)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// TestHealthHandler tests the health handler
func TestHealthHandler(t *testing.T) {
	e := echo.New()

	t.Run("Closed State Returns OK", func(t *testing.T) {
		cb := New(DefaultConfig)
		handler := cb.HealthHandler()

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err = json.NewDecoder(rec.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, string(StateClosed), response["state"])
		assert.Equal(t, true, response["healthy"])
	})

	t.Run("Open State Returns Service Unavailable", func(t *testing.T) {
		cb := New(DefaultConfig)
		cb.ForceOpen()
		handler := cb.HealthHandler()

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

		var response map[string]interface{}
		err = json.NewDecoder(rec.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, string(StateOpen), response["state"])
		assert.Equal(t, false, response["healthy"])
	})

	t.Run("Half-Open State Returns OK", func(t *testing.T) {
		cb := New(DefaultConfig)
		cb.mutex.Lock()
		cb.state = StateHalfOpen
		cb.mutex.Unlock()
		handler := cb.HealthHandler()

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err = json.NewDecoder(rec.Body).Decode(&response)
		assert.NoError(t, err)
		assert.Equal(t, string(StateHalfOpen), response["state"])
		assert.Equal(t, false, response["healthy"])
	})
}
