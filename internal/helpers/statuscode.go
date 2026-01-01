package helpers

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
)

// DefaultStatusResolver resolves http status code from given err or Response.
func DefaultStatusResolver(c *echo.Context, err error) int {
	status := 0
	var sc echo.HTTPStatusCoder
	if errors.As(err, &sc) {
		return sc.StatusCode()
	}
	if eResp, uErr := echo.UnwrapResponse(c.Response()); uErr == nil {
		if eResp.Committed {
			status = eResp.Status
		}
	}
	if err != nil && status == 0 {
		status = http.StatusInternalServerError
	}
	return status
}
