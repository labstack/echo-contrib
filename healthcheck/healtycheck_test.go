package healthcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestNewHandlerFunc(t *testing.T) {
	type args struct {
		opts []Option
	}
	tests := []struct {
		name       string
		args       []Option
		statusCode int
		response   response
	}{
		{
			name:       "returns 200 status if no errors",
			statusCode: http.StatusOK,
			response: response{
				Status: http.StatusText(http.StatusOK),
			},
		},
		{
			name:       "returns 503 status if errors",
			statusCode: http.StatusServiceUnavailable,
			args: []Option{
				WithChecker("database", CheckerFunc(func(ctx context.Context) error {
					return fmt.Errorf("connection to db timed out")
				})),
				WithChecker("testService", CheckerFunc(func(ctx context.Context) error {
					return fmt.Errorf("connection refused")
				})),
			},
			response: response{
				Status: http.StatusText(http.StatusServiceUnavailable),
				Errors: map[string]string{
					"database":    "connection to db timed out",
					"testService": "connection refused",
				},
			},
		},
		{
			name:       "returns 503 status if checkers timeout",
			statusCode: http.StatusServiceUnavailable,
			args: []Option{
				WithTimeout(1 * time.Millisecond),
				WithChecker("database", CheckerFunc(func(ctx context.Context) error {
					time.Sleep(10 * time.Millisecond)
					return nil
				})),
			},
			response: response{
				Status: http.StatusText(http.StatusServiceUnavailable),
				Errors: map[string]string{
					"database": "max check time exceeded",
				},
			},
		},
		{
			name:       "returns 200 status if errors are observable",
			statusCode: http.StatusOK,
			args: []Option{
				WithObserver("observableService", CheckerFunc(func(ctx context.Context) error {
					return fmt.Errorf("i fail but it is okay")
				})),
			},
			response: response{
				Status: http.StatusText(http.StatusOK),
				Errors: map[string]string{
					"observableService": "i fail but it is okay",
				},
			},
		},
		{
			name:       "returns 503 status if errors with observable fails",
			statusCode: http.StatusServiceUnavailable,
			args: []Option{
				WithObserver("database", CheckerFunc(func(ctx context.Context) error {
					return fmt.Errorf("connection to db timed out")
				})),
				WithChecker("testService", CheckerFunc(func(ctx context.Context) error {
					return fmt.Errorf("connection refused")
				})),
			},
			response: response{
				Status: http.StatusText(http.StatusServiceUnavailable),
				Errors: map[string]string{
					"database":    "connection to db timed out",
					"testService": "connection refused",
				},
			},
		},
		{
			name:       "returns 200 status if no errors for http check",
			statusCode: http.StatusOK,
			response: response{
				Status: http.StatusText(http.StatusOK),
			},
			args: []Option{
				WithChecker("google", HttpChecker("https://www.google.com",200,0, map[string][]string{
					"TraceId":{"ABC","DEF"},
				}) ),
			},
		},
		{
			name:       "returns 404 status if http invalid url",
			statusCode: http.StatusServiceUnavailable,
			response: response{
				Status: http.StatusText(http.StatusServiceUnavailable),
				Errors: map[string]string{
					"google":"downstream service returned unexpected status: 404",
				},
			},
			args: []Option{
				WithChecker("google", HttpChecker("https://www.google.com/noSuchUrl",200,0,nil) ),
			},
		},
		{
			name:       "returns 503 status if errors for http check",
			statusCode: http.StatusServiceUnavailable,
			response: response{
				Status: http.StatusText(http.StatusServiceUnavailable),
				Errors: map[string]string{
					"invalid_http":"error while checking: https://www.abbsdadasdads.com",
				},
			},
			args: []Option{
				WithChecker("invalid_http", HttpChecker("https://www.abbsdadasdads.com",200,0,nil) ),
			},
		},
		{
			name:       "returns 200 status if file exist",
			statusCode: http.StatusOK,
			response: response{
				Status: http.StatusText(http.StatusOK),
			},
			args: []Option{
				WithChecker("file", FileChecker("healtycheck_test.go") ),
			},
		},
		{
			name:       "returns 503 status if file doesnt exist",
			statusCode: http.StatusServiceUnavailable,
			response: response{
				Status: http.StatusText(http.StatusServiceUnavailable),
				Errors: map[string]string{
					"file": "no_file_here file doesn't exist",
				},
			},
			args: []Option{
				WithChecker("file", FileChecker("no_file_here") ),
			},
		},
		{
			name:       "returns success status tcp checker",
			statusCode: http.StatusOK,
			response: response{
				Status: http.StatusText(http.StatusOK),
			},
			args: []Option{
				WithChecker("tcp", TcpChecker("www.google.com:80",5*time.Second) ),
			},
		},
		{
			name:       "returns success status tcp checker",
			statusCode: http.StatusServiceUnavailable,
			response: response{
				Status: http.StatusText(http.StatusServiceUnavailable),
				Errors: map[string]string{
					"tcp": "connection to 127.0.0.1 failed",
				},
			},
			args: []Option{
				WithChecker("tcp", TcpChecker("127.0.0.1",5*time.Second) ),
			},
		},
	}
	e := echo.New()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			tt:= New(test.args...)
			tt.SetEndpoint("/health")
			tt.Use(e)
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			res := rec.Result()
			defer res.Body.Close()

			tt.handlerFunc(c)

			if rec.Code != test.statusCode {
				t.Errorf("expected code %d, got %d", test.statusCode, rec.Code)
			}

			var respBody response
			if err := json.NewDecoder(rec.Body).Decode(&respBody); err != nil {
				t.Errorf("failed to parse the body")
			}
			if !reflect.DeepEqual(respBody, test.response) {
				t.Errorf("NewHandlerFunc() = %v, want %v", respBody, test.response)
			}


		})
	}
}