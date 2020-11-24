Usage

```code go
package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo-contrib/pprof"

)

func main() {
	e := echo.New()
	pprof.Register(e)
    ......
	e.Logger.Fatal(e.Start(":1323"))
}
```

- Then use the pprof tool to look at the heap profile:

    `go tool pprof http://localhost:1323/debug/pprof/heap`

-  Or to look at a 30-second CPU profile:
    
    `go tool pprof http://localhost:1323/debug/pprof/profile?seconds=30`

- Or to look at the goroutine blocking profile, after calling runtime.SetBlockProfileRate in your program:
    
    `go tool pprof http://localhost:1323/debug/pprof/block`

- Or to look at the holders of contended mutexes, after calling runtime.SetMutexProfileFraction in your program:
    
    `go tool pprof http://localhost:1323/debug/pprof/mutex`


