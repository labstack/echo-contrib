package pprof

import (
	"net/http"
	"net/http/pprof"

	"github.com/labstack/echo/v4"
)

const (
	// DefaultPrefix url prefix of pprof
	DefaultPrefix = "/debug/pprof"
)

func getPrefix(prefixOptions ...string) string {
	if len(prefixOptions) > 0 {
		return prefixOptions[0]
	}
	return DefaultPrefix
}

// Register middleware for net/http/pprof
func Register(e *echo.Echo, prefixOptions ...string) {
	prefix := getPrefix(prefixOptions...)

	prefixRouter := e.Group(prefix)
	{
		prefixRouter.GET("/", handler(pprof.Index))
		prefixRouter.GET("/allocs", handler(pprof.Handler("allocs").ServeHTTP))
		prefixRouter.GET("/block", handler(pprof.Handler("block").ServeHTTP))
		prefixRouter.GET("/cmdline", handler(pprof.Cmdline))
		prefixRouter.GET("/goroutine", handler(pprof.Handler("goroutine").ServeHTTP))
		prefixRouter.GET("/heap", handler(pprof.Handler("heap").ServeHTTP))
		prefixRouter.GET("/mutex", handler(pprof.Handler("mutex").ServeHTTP))
		prefixRouter.GET("/profile", handler(pprof.Profile))
		prefixRouter.POST("/symbol", handler(pprof.Symbol))
		prefixRouter.GET("/symbol", handler(pprof.Symbol))
		prefixRouter.GET("/threadcreate", handler(pprof.Handler("threadcreate").ServeHTTP))
		prefixRouter.GET("/trace", handler(pprof.Trace))
	}
}

func handler(h http.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		h.ServeHTTP(c.Response().Writer, c.Request())
		return nil
	}
}
