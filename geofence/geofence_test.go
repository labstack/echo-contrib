package echogeofence

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/circa10a/go-geofence"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestMiddleware(t *testing.T) {
	tests := []struct {
		geofence             *geofence.Config
		name                 string
		remoteAddr           string
		expectedResponseCode int
	}{
		{
			name:                 "PrivateIPSkip",
			expectedResponseCode: 200,
			geofence: &geofence.Config{
				IPAddress:               "",
				Token:                   "fake",
				Radius:                  0,
				AllowPrivateIPAddresses: true,
				CacheTTL:                0,
			},
			remoteAddr: "192.168.1.100:80",
		},
		{
			name:                 "InvalidIPFormat",
			expectedResponseCode: 500,
			geofence: &geofence.Config{
				IPAddress: "",
				Token:     "fake",
				Radius:    0,
				CacheTTL:  0,
			},
			remoteAddr: "0.0.0.0",
		},
		{
			name:                 "InvalidToken",
			expectedResponseCode: 500,
			geofence: &geofence.Config{
				IPAddress: "",
				Token:     "fake",
				Radius:    0,
				CacheTTL:  0,
			},
			remoteAddr: "0.0.0.0:80",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := echo.New()

			geofence, _ := geofence.New(test.geofence)

			e.GET("/", func(c echo.Context) error {
				return c.String(http.StatusOK, "Hello, World!")
			}, Middleware(geofence))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			res := httptest.NewRecorder()

			if test.remoteAddr != "" {
				req.RemoteAddr = test.remoteAddr
			}

			e.ServeHTTP(res, req)
			assert.Equal(t, test.expectedResponseCode, res.Code)
		})
	}
}
