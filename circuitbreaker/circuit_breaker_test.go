package circuitbreaker

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreakerBasicOperations(t *testing.T) {
	// Create circuit breaker with custom config
	cb := New(Config{
		FailureThreshold:      3,
		Timeout:               100 * time.Millisecond,
		SuccessThreshold:      2,
		HalfOpenMaxConcurrent: 2,
	})

	// Initial state should be closed
	assert.Equal(t, StateClosed, cb.State())
	assert.False(t, cb.IsOpen())

	// Test state transitions
	t.Run("State transitions", func(t *testing.T) {
		// Reporting failures should eventually open the circuit
		for i := 0; i < 3; i++ {
			cb.ReportFailure()
		}
		assert.Equal(t, StateOpen, cb.State())
		assert.True(t, cb.IsOpen())

		// Requests should be rejected in open state
		allowed, state := cb.AllowRequest()
		assert.False(t, allowed)
		assert.Equal(t, StateOpen, state)

		// Wait for timeout to transition to half-open
		time.Sleep(150 * time.Millisecond)
		assert.Equal(t, StateHalfOpen, cb.State())

		// Some requests should be allowed in half-open state
		allowed, state = cb.AllowRequest()
		assert.True(t, allowed)
		assert.Equal(t, StateHalfOpen, state)

		// Report successes to close the circuit
		cb.ReportSuccess()
		cb.ReportSuccess()
		assert.Equal(t, StateClosed, cb.State())
	})

	// Reset the circuit breaker
	cb.Reset()
	assert.Equal(t, StateClosed, cb.State())

	t.Run("Force state changes", func(t *testing.T) {
		// Force open
		cb.ForceOpen()
		assert.Equal(t, StateOpen, cb.State())

		// Force close
		cb.ForceClose()
		assert.Equal(t, StateClosed, cb.State())
	})
}

func TestCircuitBreakerHalfOpenConcurrency(t *testing.T) {
	// Create circuit breaker that allows 2 concurrent requests in half-open
	cb := New(Config{
		FailureThreshold:      1,
		Timeout:               100 * time.Millisecond,
		SuccessThreshold:      2,
		HalfOpenMaxConcurrent: 2,
	})

	// Force into half-open state
	cb.ForceOpen()
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, StateHalfOpen, cb.State())

	// First two requests should be allowed
	allowed1, _ := cb.AllowRequest()
	allowed2, _ := cb.AllowRequest()
	assert.True(t, allowed1)
	assert.True(t, allowed2)

	// Third request should be rejected
	allowed3, _ := cb.AllowRequest()
	assert.False(t, allowed3)

	// After releasing one slot, a new request should be allowed
	cb.ReleaseHalfOpen()
	allowed4, _ := cb.AllowRequest()
	assert.True(t, allowed4)
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	cb := New(Config{
		FailureThreshold:      5,
		Timeout:               100 * time.Millisecond,
		SuccessThreshold:      3,
		HalfOpenMaxConcurrent: 2,
	})

	// Test concurrent requests
	t.Run("Concurrent requests", func(t *testing.T) {
		var wg sync.WaitGroup
		numRequests := 100

		wg.Add(numRequests)
		for i := 0; i < numRequests; i++ {
			go func(i int) {
				defer wg.Done()
				allowed, _ := cb.AllowRequest()
				if allowed && i%2 == 0 {
					cb.ReportSuccess()
				} else if allowed {
					cb.ReportFailure()
				}
			}(i)
		}

		wg.Wait()
		metrics := cb.Metrics()
		assert.Equal(t, int64(numRequests), metrics["totalRequests"])
	})
}

func TestCircuitBreakerMetrics(t *testing.T) {
	cb := New(DefaultConfig)

	// Report some activities
	cb.ReportFailure()
	allowed, _ := cb.AllowRequest()
	assert.True(t, allowed)
	cb.ReportSuccess()

	// Check basic metrics
	metrics := cb.Metrics()
	assert.Equal(t, int64(1), metrics["failures"])
	assert.Equal(t, int64(1), metrics["totalRequests"])
	assert.Equal(t, StateClosed, metrics["state"])

	// Check detailed stats
	stats := cb.GetStateStats()
	assert.Equal(t, DefaultConfig.FailureThreshold, stats["failureThreshold"])
	assert.Equal(t, DefaultConfig.SuccessThreshold, stats["successThreshold"])
	assert.Equal(t, DefaultConfig.Timeout, stats["openDuration"])
}

func TestTimestampTransitions(t *testing.T) {

	t.Skip("Skipping test for timestamp transitions")

	// Create a circuit breaker with a controlled clock for testing
	now := time.Now()
	mockClock := func() time.Time {
		return now
	}

	cb := New(Config{
		FailureThreshold:      1,
		Timeout:               5 * time.Second,
		SuccessThreshold:      1,
		HalfOpenMaxConcurrent: 1,
	})
	// Set the mock clock
	cb.now = mockClock

	// Trigger the circuit open
	cb.ReportFailure()
	assert.Equal(t, StateOpen, cb.State())

	// Verify openUntil is set properly
	stats := cb.GetStateStats()
	openUntil, ok := stats["openUntil"].(time.Time)
	require.True(t, ok)
	assert.InDelta(t, now.Add(5*time.Second).UnixNano(), openUntil.UnixNano(), float64(time.Microsecond))

	fmt.Println(now.String())

	// Advance time to just before timeout
	now = now.Add(4 * time.Second)
	fmt.Println(now.String())
	assert.Equal(t, StateOpen, cb.State())

	fmt.Println("Advance time to just before timeout:", cb.State())

	// Advance time past timeout
	now = now.Add(2 * time.Second)
	fmt.Println(now.String())
	assert.Equal(t, StateHalfOpen, cb.State())
}

func TestMiddleware(t *testing.T) {
	// Setup
	e := echo.New()
	cb := New(DefaultConfig)

	// Create a test handler that can be configured to succeed or fail
	shouldFail := false
	testHandler := func(c echo.Context) error {
		if shouldFail {
			return echo.NewHTTPError(http.StatusInternalServerError, "test error")
		}
		return c.String(http.StatusOK, "success")
	}

	// Apply middleware
	handler := Middleware(cb)(testHandler)

	t.Run("Success case", func(t *testing.T) {
		// Create request and recorder
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute request
		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		// Check metrics
		metrics := cb.Metrics()
		assert.Equal(t, int64(1), metrics["totalRequests"])
		assert.Equal(t, int64(0), metrics["failures"])
	})

	t.Run("Failure case", func(t *testing.T) {
		// Configure handler to fail
		shouldFail = true

		// Create request and recorder
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute request and expect error (which middleware passes through)
		err := handler(c)
		assert.Error(t, err)

		// Check metrics - failures should be incremented
		metrics := cb.Metrics()
		assert.Equal(t, int64(2), metrics["totalRequests"])
		assert.Equal(t, int64(1), metrics["failures"])

		// Force more failures to open the circuit
		for i := 0; i < DefaultConfig.FailureThreshold-1; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			_ = handler(c)
		}

		// Circuit should now be open
		assert.Equal(t, StateOpen, cb.State())

		// Requests should be rejected
		req = httptest.NewRequest(http.MethodGet, "/", nil)
		rec = httptest.NewRecorder()
		c = e.NewContext(req, rec)
		err = handler(c)
		assert.NoError(t, err) // OnOpen callback handles the response
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

func TestCustomCallbacks(t *testing.T) {
	callbackInvoked := false

	cb := New(Config{
		FailureThreshold:      1,
		Timeout:               100 * time.Millisecond,
		SuccessThreshold:      1,
		HalfOpenMaxConcurrent: 1,
		OnOpen: func(c echo.Context) error {
			callbackInvoked = true
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error":  "circuit open",
				"status": "unavailable",
			})
		},
	})

	// Setup Echo
	e := echo.New()
	testHandler := func(c echo.Context) error {
		return errors.New("some error")
	}
	handler := Middleware(cb)(testHandler)

	// First request opens the circuit
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_ = handler(c)

	// Second request should invoke the callback
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	_ = handler(c)

	assert.True(t, callbackInvoked)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "circuit open")
}

func TestErrorHandling(t *testing.T) {
	errorCalled := false

	// Create a logger that captures errors
	e := echo.New()
	e.Logger.SetOutput(new(testLogWriter))

	// Create circuit breaker with callbacks that return errors
	cb := New(Config{
		FailureThreshold:      1,
		Timeout:               100 * time.Millisecond,
		SuccessThreshold:      1,
		HalfOpenMaxConcurrent: 1,
		OnClose: func(c echo.Context) error {
			errorCalled = true
			return errors.New("test error in callback")
		},
	})

	// Force into half-open state
	cb.ForceOpen()
	time.Sleep(150 * time.Millisecond)

	// Create handler
	testHandler := func(c echo.Context) error {
		return nil // Success
	}
	handler := Middleware(cb)(testHandler)

	// Execute request to trigger transition to closed
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	err := handler(c)

	// The error should be logged but not returned
	assert.NoError(t, err)
	assert.True(t, errorCalled)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// Helper type for capturing logs
type testLogWriter struct{}

func (w *testLogWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}
