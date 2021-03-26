package zipkintracing

import (
	"encoding/json"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/middleware/http"
	"github.com/openzipkin/zipkin-go/propagation/b3"
	"github.com/openzipkin/zipkin-go/reporter"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	zipkinHttpReporter "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/stretchr/testify/assert"
)

type zipkinSpanRequest struct {
	ID            string
	TraceID       string
	Timestamp     uint64
	Name          string
	LocalEndpoint struct {
		ServiceName string
	}
	Tags map[string]string
}

// DefaultTracer returns zipkin tracer with defaults for testing
func DefaultTracer(reportingURL, serviceName string, tags map[string]string) (*zipkin.Tracer, reporter.Reporter, error) {
	endpoint, err := zipkin.NewEndpoint(serviceName, "")
	if err != nil {
		return nil, nil, err
	}
	reporter := zipkinHttpReporter.NewReporter(reportingURL)
	tracer, err := zipkin.NewTracer(reporter, zipkin.WithLocalEndpoint(endpoint), zipkin.WithTags(tags))
	if err != nil {
		return nil, nil, err
	}
	return tracer, reporter, nil
}

func TestDoHTTTP(t *testing.T) {
	done := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)

		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)

		var spans []zipkinSpanRequest
		err = json.Unmarshal(body, &spans)
		assert.NoError(t, err)

		assert.NotEmpty(t, spans[0].ID)
		assert.NotEmpty(t, spans[0].TraceID)
		assert.Equal(t, "http/get", spans[0].Name)
		assert.Equal(t, "echo-service", spans[0].LocalEndpoint.ServiceName)
	}))
	defer ts.Close()

	echoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, http.MethodGet)
		assert.NotEmpty(t, r.Header.Get(b3.TraceID))
		assert.NotEmpty(t, r.Header.Get(b3.SpanID))
	}))
	defer echoServer.Close()
	tracer, reporter, err := DefaultTracer(ts.URL, "echo-service", nil)
	req := httptest.NewRequest(http.MethodGet, echoServer.URL, nil)
	req.RequestURI = ""
	rec := httptest.NewRecorder()
	assert.NoError(t, err)
	e := echo.New()
	c := e.NewContext(req, rec)
	client, err := zipkinhttp.NewClient(tracer)
	assert.NoError(t, err)
	_, err = DoHTTP(c, req, client)
	assert.NoError(t, err)
	err = reporter.Close()
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Millisecond * 1500):
		t.Fatalf("Test server did not receive spans")
	}
}

func TestTraceFunc(t *testing.T) {
	done := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)

		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)

		var spans []zipkinSpanRequest
		err = json.Unmarshal(body, &spans)
		assert.NoError(t, err)
		assert.NotEmpty(t, spans[0].ID)
		assert.NotEmpty(t, spans[0].TraceID)
		assert.Equal(t, "s3_read", spans[0].Name)
		assert.Equal(t, "echo-service", spans[0].LocalEndpoint.ServiceName)
		assert.NotNil(t, spans[0].Tags["availability_zone"])
		assert.Equal(t, "us-east-1", spans[0].Tags["availability_zone"])
	}))
	defer ts.Close()
	e := echo.New()
	req := httptest.NewRequest("GET", "http://localhost:8080/echo", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	traceTags := make(map[string]string)
	traceTags["availability_zone"] = "us-east-1"
	tracer, reporter, err := DefaultTracer(ts.URL, "echo-service", traceTags)
	assert.NoError(t, err)
	s3func := func(name string) {
		TraceFunc(c, "s3_read", DefaultSpanTags, tracer)()
		assert.Equal(t, "s3Test", name)
	}
	s3func("s3Test")
	err = reporter.Close()
	assert.NoError(t, err)
	select {
	case <-done:
	case <-time.After(time.Millisecond * 15500):
		t.Fatalf("Test server did not receive spans")
	}
}

func TestTraceProxy(t *testing.T) {
	done := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)

		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)

		var spans []zipkinSpanRequest
		err = json.Unmarshal(body, &spans)
		assert.NoError(t, err)

		assert.NotEmpty(t, spans[0].ID)
		assert.NotEmpty(t, spans[0].TraceID)
		assert.Equal(t, "c get reverse proxy", spans[0].Name)
		assert.Equal(t, "echo-service", spans[0].LocalEndpoint.ServiceName)
		assert.NotNil(t, spans[0].Tags["availability_zone"])
		assert.Equal(t, "us-east-1", spans[0].Tags["availability_zone"])
	}))
	defer ts.Close()
	traceTags := make(map[string]string)
	traceTags["availability_zone"] = "us-east-1"
	tracer, reporter, err := DefaultTracer(ts.URL, "echo-service", traceTags)
	req := httptest.NewRequest("GET", "http://localhost:8080/accounts/acctrefid/transactions", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)
	mw := TraceProxy(tracer)
	h := mw(func(c echo.Context) error {
		return nil
	})
	err = h(c)
	assert.NoError(t, err)
	assert.NotEmpty(t, req.Header.Get(b3.TraceID))
	assert.NotEmpty(t, req.Header.Get(b3.SpanID))

	err = reporter.Close()
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Millisecond * 1500):
		t.Fatalf("Test server did not receive spans")
	}
}

func TestTraceServer(t *testing.T) {
	done := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)

		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)

		var spans []zipkinSpanRequest
		err = json.Unmarshal(body, &spans)
		assert.NoError(t, err)

		assert.NotEmpty(t, spans[0].ID)
		assert.NotEmpty(t, spans[0].TraceID)
		assert.Equal(t, "s get /accounts/acctrefid/transactions", spans[0].Name)
		assert.Equal(t, "echo-service", spans[0].LocalEndpoint.ServiceName)
		assert.NotNil(t, spans[0].Tags["availability_zone"])
		assert.Equal(t, "us-east-1", spans[0].Tags["availability_zone"])
	}))
	defer ts.Close()
	traceTags := make(map[string]string)
	traceTags["availability_zone"] = "us-east-1"
	tracer, reporter, err := DefaultTracer(ts.URL, "echo-service", traceTags)
	req := httptest.NewRequest("GET", "http://localhost:8080/accounts/acctrefid/transactions", nil)
	rec := httptest.NewRecorder()
	mw := TraceServer(tracer)
	h := mw(func(c echo.Context) error {
		return nil
	})
	assert.NoError(t, err)
	e := echo.New()
	c := e.NewContext(req, rec)
	err = h(c)
	err = reporter.Close()
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Millisecond * 1500):
		t.Fatalf("Test server did not receive spans")
	}
}

func TestTraceServerWithConfig(t *testing.T) {
	done := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)

		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)

		var spans []zipkinSpanRequest
		err = json.Unmarshal(body, &spans)
		assert.NoError(t, err)

		assert.NotEmpty(t, spans[0].ID)
		assert.NotEmpty(t, spans[0].TraceID)
		assert.Equal(t, "s get /accounts/acctrefid/transactions", spans[0].Name)
		assert.Equal(t, "echo-service", spans[0].LocalEndpoint.ServiceName)
		assert.NotNil(t, spans[0].Tags["availability_zone"])
		assert.Equal(t, "us-east-1", spans[0].Tags["availability_zone"])
		assert.NotNil(t, spans[0].Tags["Client-Correlation-Id"])
		assert.Equal(t, "c98404736319", spans[0].Tags["Client-Correlation-Id"])

	}))
	defer ts.Close()
	traceTags := make(map[string]string)
	traceTags["availability_zone"] = "us-east-1"
	tracer, reporter, err := DefaultTracer(ts.URL, "echo-service", traceTags)
	req := httptest.NewRequest("GET", "http://localhost:8080/accounts/acctrefid/transactions", nil)
	req.Header.Add("Client-Correlation-Id", "c98404736319")
	rec := httptest.NewRecorder()
	tags := func(c echo.Context) map[string]string {
		tags := make(map[string]string)
		correlationID := c.Request().Header.Get("Client-Correlation-Id")
		tags["Client-Correlation-Id"] = correlationID
		return tags
	}
	config := TraceServerConfig{Skipper: middleware.DefaultSkipper, SpanTags: tags, Tracer: tracer}
	mw := TraceServerWithConfig(config)
	h := mw(func(c echo.Context) error {
		return nil
	})
	assert.NoError(t, err)
	e := echo.New()
	c := e.NewContext(req, rec)
	err = h(c)
	err = reporter.Close()
	assert.NoError(t, err)
	select {
	case <-done:
	case <-time.After(time.Millisecond * 1500):
		t.Fatalf("Test server did not receive spans")
	}
}

func TestTraceServerWithConfigSkipper(t *testing.T) {
	done := make(chan struct{})
	neverCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		var spans []zipkinSpanRequest
		err = json.Unmarshal(body, &spans)
		assert.NoError(t, err)
	}))
	defer ts.Close()
	traceTags := make(map[string]string)
	tracer, reporter, err := DefaultTracer(ts.URL, "echo-service", traceTags)
	traceTags["availability_zone"] = "us-east-1"
	req := httptest.NewRequest("GET", "http://localhost:8080/health", nil)
	rec := httptest.NewRecorder()
	config := TraceServerConfig{Skipper: func(c echo.Context) bool {
		return c.Request().URL.Path == "/health"
	}, Tracer: tracer}
	mw := TraceServerWithConfig(config)
	h := mw(func(c echo.Context) error {
		return nil
	})
	assert.NoError(t, err)
	e := echo.New()
	c := e.NewContext(req, rec)
	err = h(c)
	err = reporter.Close()
	assert.NoError(t, err)
	select {
	case <-done:
	case <-time.After(time.Millisecond * 500):
		neverCalled = true
	}
	assert.True(t, neverCalled)
}

func TestStartChildSpan(t *testing.T) {
	done := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)

		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)

		var spans []zipkinSpanRequest
		err = json.Unmarshal(body, &spans)
		assert.NoError(t, err)
		assert.NotEmpty(t, spans[0].ID)
		assert.NotEmpty(t, spans[0].TraceID)
		assert.Equal(t, "kinesis-test", spans[0].Name)
		assert.Equal(t, "echo-service", spans[0].LocalEndpoint.ServiceName)
		assert.NotNil(t, spans[0].Tags["availability_zone"])
		assert.Equal(t, "us-east-1", spans[0].Tags["availability_zone"])
	}))
	defer ts.Close()
	traceTags := make(map[string]string)
	traceTags["availability_zone"] = "us-east-1"
	tracer, reporter, err := DefaultTracer(ts.URL, "echo-service", traceTags)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "http://localhost:8080/health", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)

	childSpan := StartChildSpan(c, "kinesis-test", tracer)
	time.Sleep(500)
	childSpan.Finish()
	assert.NoError(t, err)
	err = reporter.Close()
	assert.NoError(t, err)
	select {
	case <-done:
	case <-time.After(time.Millisecond * 15500):
		t.Fatalf("Test server did not receive spans")
	}
}
