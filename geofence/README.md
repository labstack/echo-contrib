# geofence

Geofencing middleware that allows you to define a physical radius for clients to access your webserver.

Powered by [go-geofence](https://github.com/circa10a/go-geofence) and [freegeoip.app](https://freegeoip.app/).

## Usage

First, you will need a free API Token from [freegeoip.app](https://freegeoip.app/).

```go
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/circa10a/go-geofence"
	echogeofence "github.com/labstack/echo-contrib/geofence"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	e.IPExtractor = echo.ExtractIPDirect()
	geofence, err := geofence.New(&geofence.Config{
		// Compare incoming traffic to the current IP address running echo. This can be a set to a different IP address to geofence.
		IPAddress: "",
		// REQUIRED Freegeoip.app API token. Free allows 15,000 requests per hour. (caching will mitigate this)
		Token: "<API TOKEN>",
		// Maximum radius of the geofence in kilometers, only clients less than or equal to this distance will be allowed. (1 kilometer)
		Radius: 1.0,
		// Allow 192.X, 172.X, 10.X and loopback addresses
		AllowPrivateIPAddresses: true
		// 1 week, -1 for indefinite (until restart)
		CacheTTL: 7 * (24 * time.Hour),
	})
	if err != nil {
		log.Fatal(err)
	}

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	}, echogeofence.Middleware(geofence))

	e.Start(":8080")
}
```

## Rejecting Requests

For clients not within the geofence radius, a `403 Forbidden` response will be returned like so:

```json
{
  "message": "Forbidden"
}
```

## Troubleshooting

If you're behind a proxy, you'll want to perform the correct IP extraction method based on your setup. See the [echo docs](https://echo.labstack.com/guide/ip-address/) for more.

To view the IP address being validated, use this logging config to view the remote ip:

```go
e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
  Format: "method=${method}, uri=${uri}, status=${status}, remote_ip=${remote_ip}\n",
}))
```
