# Usage
Simple example:
```go
	package main

	import (
		"github.com/casbin/casbin/v2"
		"github.com/labstack/echo/v4"
		casbin_mw "github.com/labstack/echo-contrib/casbin"
	)

	func main() {
		e := echo.New()

		// Mediate the access for every request
		e.Use(casbin_mw.Middleware(casbin.NewEnforcer("auth_model.conf", "auth_policy.csv")))

		e.Logger.Fatal(e.Start(":1323"))
	}
```

Advanced example:
```go
	package main

	import (
		"github.com/casbin/casbin/v2"
		"github.com/labstack/echo/v4"
		casbin_mw "github.com/labstack/echo-contrib/casbin"
	)

	func main() {
		ce, _ := casbin.NewEnforcer("auth_model.conf", "")
		ce.AddRoleForUser("alice", "admin")
		ce.AddPolicy("added_user", "data1", "read")
		
		e := echo.New()

		e.Use(casbin_mw.Middleware(ce))

		e.Logger.Fatal(e.Start(":1323"))
	}
```

# API Reference
See [API Overview](https://casbin.org/docs/api-overview).