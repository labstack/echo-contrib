package circuitbreaker

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
)

// State represents the state of the circuit breaker
type State string

const (
	StateClosed   State = "closed"    // Normal operation
	StateOpen     State = "open"      // Requests are blocked
	StateHalfOpen State = "half-open" // Limited requests allowed to check recovery
)

// Config holds the configurable parameters
type Config struct {
	// Failure threshold to trip the circuit
	FailureThreshold int
	// Duration circuit stays open before allowing test requests
	Timeout time.Duration
	// Success threshold to close the circuit from half-open
	SuccessThreshold int
	// Maximum concurrent requests allowed in half-open state
	HalfOpenMaxConcurrent int64
	// Custom failure detector function (return true if response should count as failure)
	IsFailure func(c echo.Context, err error) bool
	// Callbacks for state transitions
	OnOpen     func(echo.Context) error // Called when circuit opens
	OnHalfOpen func(echo.Context) error // Called when circuit transitions to half-open
	OnClose    func(echo.Context) error // Called when circuit closes
}

// DefaultConfig provides sensible defaults for the circuit breaker
var DefaultConfig = Config{
	FailureThreshold:      5,
	Timeout:               5 * time.Second,
	SuccessThreshold:      1,
	HalfOpenMaxConcurrent: 1,
	IsFailure: func(c echo.Context, err error) bool {
		return err != nil || c.Response().Status >= http.StatusInternalServerError
	},
	OnOpen: func(c echo.Context) error {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"error": "service unavailable",
		})
	},
	OnHalfOpen: func(c echo.Context) error {
		return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
			"error": "service under recovery",
		})
	},
	OnClose: func(c echo.Context) error {
		return nil
	},
}

// HalfOpenLimiter manages concurrent requests in half-open state
type HalfOpenLimiter struct {
	maxConcurrent int64
	current       atomic.Int64
}

// NewHalfOpenLimiter creates a new limiter for half-open state
func NewHalfOpenLimiter(maxConcurrent int64) *HalfOpenLimiter {
	return &HalfOpenLimiter{
		maxConcurrent: maxConcurrent,
	}
}

// TryAcquire attempts to acquire a slot in the limiter
// Returns true if successful, false if at capacity
func (l *HalfOpenLimiter) TryAcquire() bool {
	for {
		current := l.current.Load()
		if current >= l.maxConcurrent {
			return false
		}

		if l.current.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

// Release releases a previously acquired slot
func (l *HalfOpenLimiter) Release() {
	current := l.current.Load()
	if current > 0 {
		l.current.CompareAndSwap(current, current-1)
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	failureCount     atomic.Int64     // Count of failures
	successCount     atomic.Int64     // Count of successes in half-open state
	totalRequests    atomic.Int64     // Count of total requests
	rejectedRequests atomic.Int64     // Count of rejected requests
	state            State            // Current state of circuit breaker
	mutex            sync.RWMutex     // Protects state transitions
	failureThreshold int              // Max failures before opening circuit
	timeout          time.Duration    // Duration to stay open before transitioning to half-open
	successThreshold int              // Successes required to close circuit
	openUntil        atomic.Int64     // Unix timestamp (nanos) when open state expires
	config           Config           // Configuration settings
	now              func() time.Time // Function for getting current time (useful for testing)
	halfOpenLimiter  *HalfOpenLimiter // Controls limited requests in half-open state
	lastStateChange  time.Time        // Time of last state change
}

// New initializes a circuit breaker with the given configuration
func New(config Config) *CircuitBreaker {
	// Apply default values for zero values
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = DefaultConfig.FailureThreshold
	}
	if config.Timeout <= 0 {
		config.Timeout = DefaultConfig.Timeout
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = DefaultConfig.SuccessThreshold
	}
	if config.HalfOpenMaxConcurrent <= 0 {
		config.HalfOpenMaxConcurrent = DefaultConfig.HalfOpenMaxConcurrent
	}
	if config.IsFailure == nil {
		config.IsFailure = DefaultConfig.IsFailure
	}
	if config.OnOpen == nil {
		config.OnOpen = DefaultConfig.OnOpen
	}
	if config.OnHalfOpen == nil {
		config.OnHalfOpen = DefaultConfig.OnHalfOpen
	}
	if config.OnClose == nil {
		config.OnClose = DefaultConfig.OnClose
	}

	now := time.Now()

	return &CircuitBreaker{
		failureThreshold: config.FailureThreshold,
		timeout:          config.Timeout,
		successThreshold: config.SuccessThreshold,
		state:            StateClosed,
		config:           config,
		now:              time.Now,
		halfOpenLimiter:  NewHalfOpenLimiter(config.HalfOpenMaxConcurrent),
		lastStateChange:  now,
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() State {
	// Check for auto-transition from open to half-open based on timestamp
	if cb.state == StateOpen {
		openUntil := cb.openUntil.Load()
		if openUntil > 0 && time.Now().UnixNano() >= openUntil {
			cb.transitionToHalfOpen()
		}
	}

	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// IsOpen returns true if the circuit is open
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.State() == StateOpen
}

// Reset resets the circuit breaker to its initial closed state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	// Reset counters
	cb.failureCount.Store(0)
	cb.successCount.Store(0)

	// Reset state
	cb.state = StateClosed
	cb.lastStateChange = cb.now()
	cb.openUntil.Store(0)
}

// ForceOpen forcibly opens the circuit regardless of failure count
func (cb *CircuitBreaker) ForceOpen() {
	cb.transitionToOpen()
}

// ForceClose forcibly closes the circuit regardless of current state
func (cb *CircuitBreaker) ForceClose() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.state = StateClosed
	cb.lastStateChange = cb.now()
	cb.failureCount.Store(0)
	cb.successCount.Store(0)
	cb.openUntil.Store(0)
}

// SetTimeout updates the timeout duration
func (cb *CircuitBreaker) SetTimeout(timeout time.Duration) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.timeout = timeout
}

// transitionToOpen changes state to open and sets timestamp for half-open transition
func (cb *CircuitBreaker) transitionToOpen() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if cb.state == StateOpen {
		return
	}

	cb.state = StateOpen
	cb.lastStateChange = cb.now()

	// Set timestamp when the circuit should transition to half-open
	openUntil := cb.now().Add(cb.timeout).UnixNano()
	cb.openUntil.Store(openUntil)

	// Reset failure counter
	cb.failureCount.Store(0)
}

// transitionToHalfOpen changes state from open to half-open
func (cb *CircuitBreaker) transitionToHalfOpen() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if cb.state == StateOpen {
		cb.state = StateHalfOpen
		cb.lastStateChange = cb.now()

		// Reset counters
		cb.failureCount.Store(0)
		cb.successCount.Store(0)
		cb.openUntil.Store(0)
	}
}

// transitionToClosed changes state from half-open to closed
func (cb *CircuitBreaker) transitionToClosed() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.lastStateChange = cb.now()

		// Reset counters
		cb.failureCount.Store(0)
		cb.successCount.Store(0)
	}
}

// AllowRequest determines if a request is allowed based on circuit state
func (cb *CircuitBreaker) AllowRequest() (bool, State) {
	cb.totalRequests.Add(1)

	// Check for automatic transition from open to half-open
	if cb.state == StateOpen {
		openUntil := cb.openUntil.Load()
		if openUntil > 0 && time.Now().UnixNano() >= openUntil {
			cb.transitionToHalfOpen()
		}
	}

	cb.mutex.RLock()
	state := cb.state

	var allowed bool
	switch state {
	case StateOpen:
		allowed = false
	case StateHalfOpen:
		allowed = cb.halfOpenLimiter.TryAcquire()
	default: // StateClosed
		allowed = true
	}
	cb.mutex.RUnlock()

	if !allowed {
		cb.rejectedRequests.Add(1)
	}

	return allowed, state
}

// ReleaseHalfOpen releases a slot in the half-open limiter
func (cb *CircuitBreaker) ReleaseHalfOpen() {
	if cb.State() == StateHalfOpen {
		cb.halfOpenLimiter.Release()
	}
}

// ReportSuccess increments success count and closes circuit if threshold met
func (cb *CircuitBreaker) ReportSuccess() {
	if cb.State() == StateHalfOpen {
		newSuccessCount := cb.successCount.Add(1)
		if int(newSuccessCount) >= cb.successThreshold {
			cb.transitionToClosed()
		}
	}
}

// ReportFailure increments failure count and opens circuit if threshold met
func (cb *CircuitBreaker) ReportFailure() {
	state := cb.State()

	switch state {
	case StateHalfOpen:
		// In half-open, a single failure trips the circuit
		cb.transitionToOpen()
	case StateClosed:
		newFailureCount := cb.failureCount.Add(1)
		if int(newFailureCount) >= cb.failureThreshold {
			cb.transitionToOpen()
		}
	}
}

// Metrics returns basic metrics about the circuit breaker
func (cb *CircuitBreaker) Metrics() map[string]interface{} {
	return map[string]interface{}{
		"state":            cb.State(),
		"failures":         cb.failureCount.Load(),
		"successes":        cb.successCount.Load(),
		"totalRequests":    cb.totalRequests.Load(),
		"rejectedRequests": cb.rejectedRequests.Load(),
	}
}

// GetStateStats returns detailed statistics about the circuit breaker
func (cb *CircuitBreaker) GetStateStats() map[string]interface{} {
	state := cb.State()
	openUntil := cb.openUntil.Load()

	stats := map[string]interface{}{
		"state":            state,
		"failures":         cb.failureCount.Load(),
		"successes":        cb.successCount.Load(),
		"totalRequests":    cb.totalRequests.Load(),
		"rejectedRequests": cb.rejectedRequests.Load(),
		"lastStateChange":  cb.lastStateChange,
		"openDuration":     cb.timeout,
		"failureThreshold": cb.failureThreshold,
		"successThreshold": cb.successThreshold,
	}

	if openUntil > 0 {
		stats["openUntil"] = time.Unix(0, openUntil)
	}

	return stats
}

// Middleware wraps the echo handler with circuit breaker logic
func Middleware(cb *CircuitBreaker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			allowed, state := cb.AllowRequest()

			if !allowed {
				// Call appropriate callback based on state
				if state == StateHalfOpen && cb.config.OnHalfOpen != nil {
					return cb.config.OnHalfOpen(c)
				} else if state == StateOpen && cb.config.OnOpen != nil {
					return cb.config.OnOpen(c)
				}
				return c.NoContent(http.StatusServiceUnavailable)
			}

			// If request allowed in half-open state, ensure limiter is released
			halfOpen := state == StateHalfOpen
			if halfOpen {
				defer cb.ReleaseHalfOpen()
			}

			// Execute the request
			err := next(c)

			// Check if the response should be considered a failure
			if cb.config.IsFailure(c, err) {
				cb.ReportFailure()
			} else {
				cb.ReportSuccess()

				// If transition to closed state just happened, trigger callback
				if halfOpen && cb.State() == StateClosed && cb.config.OnClose != nil {
					if closeErr := cb.config.OnClose(c); closeErr != nil {
						// Log the error but don't override the actual response
						c.Logger().Errorf("Circuit breaker OnClose callback error: %v", closeErr)
					}
				}
			}

			return err
		}
	}
}
