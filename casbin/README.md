# Usage
Simple example:
```go
package main

import (
	"log/slog"

	"github.com/casbin/casbin/v2"
	casbin_mw "github.com/labstack/echo-contrib/v5/casbin"
	"github.com/labstack/echo/v5"
)

func main() {
	e := echo.New()

	// Mediate the access for every request
	enforcer, err := casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")
	if err != nil {
		slog.Error("failed to load casbin enforcer", "error", err)
	}
	e.Use(casbin_mw.Middleware(enforcer))

	if err := e.Start(":1323"); err != nil {
		slog.Error("failed to start server", "error", err)
	}
}

```

Advanced example:
```go
package main

import (
	"log/slog"

	"github.com/casbin/casbin/v2"
	casbin_mw "github.com/labstack/echo-contrib/v5/casbin"
	"github.com/labstack/echo/v5"
)

func main() {
	ce, _ := casbin.NewEnforcer("auth_model.conf", "")
	ce.AddRoleForUser("alice", "admin")
	ce.AddPolicy(...)

	e := echo.New()

	e.Use(casbin_mw.Middleware(ce))

	if err := e.Start(":1323"); err != nil {
		slog.Error("failed to start server", "error", err)
	}
}
```

# API Reference
See [API Overview](https://casbin.org/docs/api-overview).