# Usage

```go
package main

import (
	"log/slog"

	"github.com/labstack/echo-contrib/v5/echoprometheus"
	"github.com/labstack/echo/v5"
)

func main() {
	e := echo.New()
	
	// Enable metrics middleware
	e.Use(echoprometheus.NewMiddleware("myapp"))
	e.GET("/metrics", echoprometheus.NewHandler())

	if err := e.Start(":1323"); err != nil {
		slog.Error("failed to start server", "error", err)
	}
}
```
