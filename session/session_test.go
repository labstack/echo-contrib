package session

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestMiddleware(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(echo.GET, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	handler := func(c echo.Context) error {
		sess, _ := Get("test", c)
		sess.Options.Domain = "labstack.com"
		sess.Values["foo"] = "bar"
		if err := sess.Save(c.Request(), c.Response()); err != nil {
			return err
		}
		return c.String(http.StatusOK, "test")
	}
	store := sessions.NewCookieStore([]byte("secret"))
	config := Config{
		Skipper: func(c echo.Context) bool {
			return true
		},
		Store: store,
	}

	// Skipper
	mw := MiddlewareWithConfig(config)
	h := mw(echo.NotFoundHandler)
	assert.Error(t, h(c)) // 404
	assert.Nil(t, c.Get(key))

	// Panic
	config.Skipper = nil
	config.Store = nil
	assert.Panics(t, func() {
		MiddlewareWithConfig(config)
	})

	// Core
	mw = Middleware(store)
	h = mw(handler)
	assert.NoError(t, h(c))
	assert.Contains(t, rec.Header().Get(echo.HeaderSetCookie), "labstack.com")

}

func TestGetSessionMissingStore(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(echo.GET, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	_, err := Get("test", c)

	assert.EqualError(t, err, fmt.Sprintf("%q session store not found", key))
}
