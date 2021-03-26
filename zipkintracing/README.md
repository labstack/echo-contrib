# Tracing Library for Go

This library provides tracing for go using [Zipkin](https://zipkin.io/)
 
## Usage

### Server Tracing Middleware & http client tracing

```go
package main

import (
	"github.com/labstack/echo-contrib/zipkintracing"
	"github.com/labstack/echo/v4"
	"github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/middleware/http"
	zipkinHttpReporter "github.com/openzipkin/zipkin-go/reporter/http"
	"io/ioutil"
	"net/http"
)

func main() {
	e := echo.New()
	endpoint, err := zipkin.NewEndpoint("echo-service", "")
	if err != nil {
		e.Logger.Fatalf("error creating zipkin endpoint: %s", err.Error())
	}
	reporter := zipkinHttpReporter.NewReporter("http://localhost:9411/api/v2/spans")
	traceTags := make(map[string]string)
	traceTags["availability_zone"] = "us-east-1"
	tracer, err := zipkin.NewTracer(reporter, zipkin.WithLocalEndpoint(endpoint), zipkin.WithTags(traceTags))
	client, _ := zipkinhttp.NewClient(tracer, zipkinhttp.ClientTrace(true))
	if err != nil {
		e.Logger.Fatalf("tracing init failed: %s", err.Error())
	}
	//Wrap & Use trace server middleware, this traces all server calls
	e.Use(zipkintracing.TraceServer(tracer))
	//....
	e.GET("/echo", func(c echo.Context) error {
		//trace http request calls.
		req, _ := http.NewRequest("GET", "https://echo.labstack.com/", nil)
		resp, _ := zipkintracing.DoHTTP(c, req, client)
		body, _ := ioutil.ReadAll(resp.Body)
		return c.String(http.StatusOK, string(body))
	})

	defer reporter.Close() //defer close reporter
	e.Logger.Fatal(e.Start(":8080"))
}
```
### Reverse Proxy Tracing

```go
package main

import (
	"github.com/labstack/echo-contrib/zipkintracing"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/openzipkin/zipkin-go"
	zipkinHttpReporter "github.com/openzipkin/zipkin-go/reporter/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	e := echo.New()
	//new tracing instance
	endpoint, err := zipkin.NewEndpoint("echo-service", "")
	if err != nil {
		e.Logger.Fatalf("error creating zipkin endpoint: %s", err.Error())
	}
	reporter := zipkinHttpReporter.NewReporter("http://localhost:9411/api/v2/spans")
	traceTags := make(map[string]string)
	traceTags["availability_zone"] = "us-east-1"
	tracer, err := zipkin.NewTracer(reporter, zipkin.WithLocalEndpoint(endpoint), zipkin.WithTags(traceTags))
	if err != nil {
		e.Logger.Fatalf("tracing init failed: %s", err.Error())
	}
	//....
	e.GET("/echo",  func(c echo.Context) error {
		proxyURL, _ := url.Parse("https://echo.labstack.com/")
		httputil.NewSingleHostReverseProxy(proxyURL)
		return nil
	}, zipkintracing.TraceProxy(tracer))

	defer reporter.Close() //close reporter
	e.Logger.Fatal(e.Start(":8080"))

}
```

### Trace function calls

To trace function calls e.g. to trace `s3Func`

```go
package main

import (
	"github.com/labstack/echo-contrib/zipkintracing"
	"github.com/labstack/echo/v4"
	"github.com/openzipkin/zipkin-go"
)

func s3Func(c echo.Context, tracer *zipkin.Tracer) {
	defer zipkintracing.TraceFunc(c, "s3_read", zipkintracing.DefaultSpanTags, tracer)()
	//s3Func logic here...
}
```

### Create Child Span

```go
package main

import (
	"github.com/labstack/echo-contrib/zipkintracing"
	"github.com/labstack/echo/v4"
	"github.com/openzipkin/zipkin-go"
)

func traceWithChildSpan(c echo.Context, tracer *zipkin.Tracer) {
	span := zipkintracing.StartChildSpan(c, "someMethod", tracer)
	//func logic.....
	span.Finish()
}
```