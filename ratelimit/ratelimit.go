// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2025 LabStack and Echo contributors

package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type (
	// Config defines the config for RateLimit middleware.
	Config struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// Limit is the maximum number of requests allowed within the defined window.
		// Required.
		Limit int

		// Window defines the time window for the rate limit (in seconds).
		// Default is 60 seconds (1 minute).
		Window time.Duration

		// KeyExtractor is a function used to generate a key for each request.
		// Default implementation uses the client IP address.
		KeyExtractor func(c echo.Context) string

		// ErrorHandler is a function to handle errors returned by the middleware.
		ErrorHandler func(c echo.Context, err error) error

		// ExceedHandler is a function called when rate limit is exceeded.
		// Default returns 429 Too Many Requests.
		ExceedHandler func(c echo.Context) error
	}

	// Store is an interface for storing rate limit data
	Store interface {
		// Increment increments the count for a key and returns the current count
		Increment(key string, window time.Duration) (int, error)

		// Get returns the current count for a key
		Get(key string) (int, error)

		// Cleanup removes expired entries
		Cleanup()
	}

	// MemoryStore implements in-memory storage for rate limiting
	MemoryStore struct {
		entries map[string]*entry
		mu      sync.RWMutex
	}

	entry struct {
		count    int
		expireAt time.Time
	}
)

var (
	// DefaultConfig is the default RateLimit middleware config.
	DefaultConfig = Config{
		Skipper: middleware.DefaultSkipper,
		Window:  60 * time.Second, // 1 minute
		KeyExtractor: func(c echo.Context) string {
			return c.RealIP()
		},
		ErrorHandler: func(c echo.Context, err error) error {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		},
		ExceedHandler: func(c echo.Context) error {
			return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
		},
	}

	// DefaultStore is the default in-memory store for rate limiting
	DefaultStore Store
)

// NewMemoryStore creates a new in-memory store for rate limiting
func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{
		entries: make(map[string]*entry),
	}

	go func() {
		// Clean up expired entries every minute
		for {
			time.Sleep(time.Minute)
			store.Cleanup()
		}
	}()

	return store
}

// Increment increments the count for a key and returns the current count
func (s *MemoryStore) Increment(key string, window time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if s.entries == nil {
		s.entries = make(map[string]*entry)
	}

	e, exists := s.entries[key]
	if !exists || now.After(e.expireAt) {
		s.entries[key] = &entry{
			count:    1,
			expireAt: now.Add(window),
		}
		return 1, nil
	}

	e.count++
	return e.count, nil
}

// Get returns the current count for a key
func (s *MemoryStore) Get(key string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	e, exists := s.entries[key]
	if !exists {
		return 0, nil
	}

	if now.After(e.expireAt) {
		return 0, nil
	}

	return e.count, nil
}

// Cleanup removes expired entries from the memory store
func (s *MemoryStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, e := range s.entries {
		if now.After(e.expireAt) {
			delete(s.entries, key)
		}
	}
}

// Initialize the default store
func init() {
	DefaultStore = NewMemoryStore()
}

// Middleware returns a RateLimit middleware.
func Middleware(limit int) echo.MiddlewareFunc {
	c := DefaultConfig
	c.Limit = limit
	return MiddlewareWithConfig(c)
}

// MiddlewareWithConfig returns a RateLimit middleware with config.
func MiddlewareWithConfig(config Config) echo.MiddlewareFunc {
	// Defaults
	if config.Skipper == nil {
		config.Skipper = DefaultConfig.Skipper
	}
	if config.Window == 0 {
		config.Window = DefaultConfig.Window
	}
	if config.KeyExtractor == nil {
		config.KeyExtractor = DefaultConfig.KeyExtractor
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = DefaultConfig.ErrorHandler
	}
	if config.ExceedHandler == nil {
		config.ExceedHandler = DefaultConfig.ExceedHandler
	}
	if config.Limit <= 0 {
		panic("echo: rate limit middleware requires limit > 0")
	}

	store := DefaultStore

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			key := config.KeyExtractor(c)
			count, err := store.Increment(key, config.Window)
			if err != nil {
				return config.ErrorHandler(c, err)
			}

			// Set rate limit headers
			c.Response().Header().Set("X-RateLimit-Limit", string(rune(config.Limit)))
			c.Response().Header().Set("X-RateLimit-Remaining", string(rune(config.Limit-count)))

			if count > config.Limit {
				return config.ExceedHandler(c)
			}

			return next(c)
		}
	}
}

// WithStore returns a RateLimit middleware with a custom store.
func WithStore(store Store) echo.MiddlewareFunc {
	DefaultStore = store
	return Middleware(DefaultConfig.Limit)
}

// WithStoreAndConfig returns a RateLimit middleware with a custom store and config.
func WithStoreAndConfig(store Store, config Config) echo.MiddlewareFunc {
	DefaultStore = store
	return MiddlewareWithConfig(config)
}
