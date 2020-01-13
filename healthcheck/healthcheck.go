/*
Package health check provides to add echo instance.

Example:
```
package main
import (
    "github.com/labstack/echo/v4"
    "github.com/labstack/echo-contrib/healthcheck"
)

func main() {
    e := echo.New()

	opts:=[]healthcheck.Option{
		healthcheck.WithTimeout(5*time.Second),
		healthcheck.WithChecker("call",healthcheck.HttpChecker("https://www.google.com",200,0,nil)),
		healthcheck.WithObserver("call",healthcheck.TcpChecker("127.0.0.1",5*time.Second)),
		healthcheck.WithObserver("fileX",healthcheck.FileChecker("abc")),
	}

	h:=healthcheck.New(opts...).SetEndpoint("status")
	h.Use(e)
    e.Logger.Fatal(e.Start(":1323"))
}
```
Check the status:
```
curl -X GET http://localhost:5000/status
```
*/
package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

var(
	ErrMaxCheckTimeExceededError = errors.New("max check time exceeded")

	defaultStatusEndpoint="/status"
)

type response struct {
	Status string            `json:"status,omitempty"`
	Errors map[string]string `json:"errors,omitempty"`
}

type Health struct {
	statusEndpoint					string
	checkers  						map[string]Checker
	observers 						map[string]Checker
	timeout   						time.Duration
}

type Checker interface {
	Check(ctx context.Context) error
}
type CheckerFunc func(ctx context.Context) error
func (c CheckerFunc) Check(ctx context.Context) error {
	return c(ctx)
}
func New(opts ...Option) *Health {

	h := &Health{
		statusEndpoint:defaultStatusEndpoint,
		checkers:  make(map[string]Checker),
		observers: make(map[string]Checker),
		timeout:   30 * time.Second,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}
func (h *Health) Use(e *echo.Echo) {

	e.GET(h.statusEndpoint, h.handlerFunc)
}
func (h *Health) SetEndpoint(statusEndpoint string) *Health {

	h.statusEndpoint=statusEndpoint
	return h
}
type Option func(*Health)
func WithChecker(name string, s Checker) Option {
	return func(h *Health) {
		h.checkers[name] = &timeoutChecker{s}
	}
}
func WithObserver(name string, s Checker) Option {
	return func(h *Health) {
		h.observers[name] = &timeoutChecker{s}
	}
}
func WithTimeout(timeout time.Duration) Option {
	return func(h *Health) {
		h.timeout = timeout
	}
}

func (h *Health) handlerFunc(c echo.Context) error{


	nCheckers := len(h.checkers) + len(h.observers)

	code := http.StatusOK
	errorMsgs := make(map[string]string, nCheckers)

	ctx, cancel := context.Background(), func() {}
	if h.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, h.timeout)
	}
	defer cancel()

	var mutex sync.Mutex
	var wg sync.WaitGroup
	wg.Add(nCheckers)

	for key, checker := range h.checkers {
		go func(key string, checker Checker) {
			if err := checker.Check(ctx); err != nil {
				mutex.Lock()
				errorMsgs[key] = err.Error()
				code = http.StatusServiceUnavailable
				mutex.Unlock()
			}
			wg.Done()
		}(key, checker)
	}
	for key, observer := range h.observers {
		go func(key string, observer Checker) {
			if err := observer.Check(ctx); err != nil {
				mutex.Lock()
				errorMsgs[key] = err.Error()
				mutex.Unlock()
			}
			wg.Done()
		}(key, observer)
	}
	wg.Wait()
	payload := response{
		Status: http.StatusText(code),
		Errors: errorMsgs,
	}

	return c.JSON(code,payload)

}

type timeoutChecker struct {
	checker Checker
}

func (t *timeoutChecker) Check(ctx context.Context) error {
	checkerChan := make(chan error)
	go func() {
		checkerChan <- t.checker.Check(ctx)
	}()
	select {
	case err := <-checkerChan:
		return err
	case <-ctx.Done():
		return ErrMaxCheckTimeExceededError
	}
}

func FileChecker(f string) CheckerFunc {

	return func(ctx context.Context) error {

		if _, err := os.Stat(f); err != nil {
			return errors.New(fmt.Sprintf("%s file doesn't exist", f))
		}
		return nil
	}
}

func HttpChecker(url string, statusCode int, timeout time.Duration, headers http.Header) CheckerFunc {

	return func(ctx context.Context) error {

		client := http.Client{
			Timeout: timeout,
		}
		req, err := http.NewRequest("HEAD", url, nil)
		if err != nil {
			return errors.New("error creating request: " + url)
		}
		for headerName, headerValues := range headers {
			for _, headerValue := range headerValues {
				req.Header.Add(headerName, headerValue)
			}
		}
		response, err := client.Do(req)
		if err != nil {
			return errors.New("error while checking: " + url)
		}
		if response.StatusCode != statusCode {
			return errors.New("downstream service returned unexpected status: " + strconv.Itoa(response.StatusCode))
		}
		return nil

	}
}

func TcpChecker(addr string, timeout time.Duration) CheckerFunc{
	return func(ctx context.Context) error {

		conn, err := net.DialTimeout("tcp", addr, timeout)

		if err != nil {
			return errors.New("connection to " + addr + " failed")
		}

		conn.Close()
		return nil
	}
}