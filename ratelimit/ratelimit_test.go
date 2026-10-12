// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2025 LabStack and Echo contributors

package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit(t *testing.T) {
	e := echo.New()
	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	}

	// Create rate limiter middleware with limit of 3 requests
	limiter := Middleware(3)
	h := limiter(handler)

	// Reset the default store for testing
	DefaultStore = NewMemoryStore()

	// Create a test request with IP 192.0.2.1
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"

	// First request should pass
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Second request should pass
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	err = h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Third request should pass
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	err = h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Fourth request should fail with 429
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	err = h(c)
	he, ok := err.(*echo.HTTPError)
	assert.True(t, ok)
	assert.Equal(t, http.StatusTooManyRequests, he.Code)
}

func TestRateLimitWithCustomConfig(t *testing.T) {
	e := echo.New()
	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	}

	// Custom config with shorter window and custom key extractor
	config := Config{
		Limit:  2,
		Window: 100 * time.Millisecond,
		KeyExtractor: func(c echo.Context) string {
			return "test-key"
		},
	}

	limiter := MiddlewareWithConfig(config)
	h := limiter(handler)

	// Reset the default store for testing
	DefaultStore = NewMemoryStore()

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// First request should pass
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Second request should pass
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	err = h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Third request should fail
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	err = h(c)
	he, ok := err.(*echo.HTTPError)
	assert.True(t, ok)
	assert.Equal(t, http.StatusTooManyRequests, he.Code)

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Request after window expiry should pass
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	err = h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSkipper(t *testing.T) {
	e := echo.New()
	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	}

	// Custom config with skipper
	config := Config{
		Limit: 1,
		Skipper: func(c echo.Context) bool {
			return c.Path() == "/skip"
		},
	}

	limiter := MiddlewareWithConfig(config)
	h := limiter(handler)

	// Reset the default store for testing
	DefaultStore = NewMemoryStore()

	// First request should pass
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	err := h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Second request should fail with 429
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	err = h(c)
	he, ok := err.(*echo.HTTPError)
	assert.True(t, ok)
	assert.Equal(t, http.StatusTooManyRequests, he.Code)

	// Request to skipped path should always pass
	req = httptest.NewRequest(http.MethodGet, "/skip", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetPath("/skip")
	err = h(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMemoryStoreCleanup(t *testing.T) {
	store := NewMemoryStore()

	// Add an entry that will expire soon
	_, err := store.Increment("test-key", 50*time.Millisecond)
	assert.NoError(t, err)

	// Verify the entry exists
	count, err := store.Get("test-key")
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Manually trigger cleanup
	store.Cleanup()

	// Verify the entry is removed
	count, err = store.Get("test-key")
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

// Mock store for testing custom stores
type mockStore struct {
	counts map[string]int
}

func newMockStore() *mockStore {
	return &mockStore{
		counts: make(map[string]int),
	}
}

func (s *mockStore) Increment(key string, _ time.Duration) (int, error) {
	s.counts[key]++
	return s.counts[key], nil
}

func (s *mockStore) Get(key string) (int, error) {
	return s.counts[key], nil
}

func (s *mockStore) Cleanup() {
	// No-op for mock
}
