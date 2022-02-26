package echogeofence

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// IsAddressNearChecker is an interface that guarantees the middleware is compatible with multiple libraries
// Currently satisfied with the go-geofence library
type IsAddressNearChecker interface {
	IsIPAddressNear(ipAddress string) (bool, error)
}

// Middleware looks up the coordinates of a client to see if it's nearby
func Middleware(geofence IsAddressNearChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get client ip
			ipAddress := c.RealIP()

			// Check if IP address is within defined radius
			isAllowed, err := geofence.IsIPAddressNear(ipAddress)
			if err != nil {
				return err
			}

			// Return forbidden if not within defined radius
			if !isAllowed {
				return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
			}

			// If close enough, allow to proceed
			return next(c)
		}
	}
}
