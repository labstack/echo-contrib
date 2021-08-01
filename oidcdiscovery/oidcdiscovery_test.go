package oidcdiscovery

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/xenitab/dispans/server"
	"golang.org/x/oauth2"
)

func TestHandler(t *testing.T) {
	op := server.NewTesting(t)
	defer op.Close(t)

	handler := testGetEchoHandler(t)

	e := echo.New()
	h := middleware.JWTWithConfig(middleware.JWTConfig{
		ParseTokenFunc: New(Options{
			Issuer:            op.GetURL(t),
			RequiredAudience:  "test-client",
			RequiredTokenType: "JWT+AT",
		}),
	})(handler)

	// Test without authentication
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/", nil)
	recNoAuth := httptest.NewRecorder()
	cNoAuth := e.NewContext(reqNoAuth, recNoAuth)

	err := h(cNoAuth)
	require.Error(t, err)

	// Test with authentication
	token := op.GetToken(t)
	testHandlerWithAuthentication(t, token, h, e)
	testHandlerWithIDTokenFailure(t, token, h, e)

	// Test with rotated key
	op.RotateKeys(t)
	tokenWithRotatedKey := op.GetToken(t)
	testHandlerWithAuthentication(t, tokenWithRotatedKey, h, e)
}

func BenchmarkHandler(b *testing.B) {
	op := server.NewTesting(b)
	defer op.Close(b)

	handler := testGetEchoHandler(b)

	e := echo.New()
	h := middleware.JWTWithConfig(middleware.JWTConfig{
		ParseTokenFunc: New(Options{
			Issuer: op.GetURL(b),
		}),
	})(handler)

	concurrencyLevels := []int{5, 10, 20, 50}
	for _, clients := range concurrencyLevels {
		b.Run(fmt.Sprintf("%d_clients", clients), func(b *testing.B) {
			var tokens []*oauth2.Token
			for i := 0; i < b.N; i++ {
				tokens = append(tokens, op.GetToken(b))
			}

			b.ResetTimer()

			var wg sync.WaitGroup
			ch := make(chan int, clients)
			for i := 0; i < b.N; i++ {
				token := tokens[i]
				wg.Add(1)
				ch <- 1
				go func() {
					defer wg.Done()
					testHandlerWithAuthentication(b, token, h, e)
					<-ch
				}()
			}
			wg.Wait()
		})
	}
}

func BenchmarkHandlerRequirements(b *testing.B) {
	op := server.NewTesting(b)
	defer op.Close(b)

	handler := testGetEchoHandler(b)

	e := echo.New()
	h := middleware.JWTWithConfig(middleware.JWTConfig{
		ParseTokenFunc: New(Options{
			Issuer:            op.GetURL(b),
			RequiredTokenType: "JWT+AT",
			RequiredAudience:  "test-client",
			RequiredClaims: map[string]interface{}{
				"sub": "test",
			},
		}),
	})(handler)

	concurrencyLevels := []int{5, 10, 20, 50}
	for _, clients := range concurrencyLevels {
		b.Run(fmt.Sprintf("%d_clients", clients), func(b *testing.B) {
			var tokens []*oauth2.Token
			for i := 0; i < b.N; i++ {
				tokens = append(tokens, op.GetToken(b))
			}

			b.ResetTimer()

			var wg sync.WaitGroup
			ch := make(chan int, clients)
			for i := 0; i < b.N; i++ {
				token := tokens[i]
				wg.Add(1)
				ch <- 1
				go func() {
					defer wg.Done()
					testHandlerWithAuthentication(b, token, h, e)
					<-ch
				}()
			}
			wg.Wait()
		})
	}
}

func BenchmarkHandlerHttp(b *testing.B) {
	op := server.NewTesting(b)
	defer op.Close(b)

	handler := testGetEchoHandler(b)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		err := e.Shutdown(ctx)
		require.NoError(b, err)
	}()

	e.Use(middleware.JWTWithConfig(middleware.JWTConfig{
		ParseTokenFunc: New(Options{
			Issuer: op.GetURL(b),
		}),
	}))

	e.GET("/", handler)

	port, err := freeport.GetFreePort()
	require.NoError(b, err)

	addr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port))
	urlString := fmt.Sprintf("http://%s/", addr)

	go func() {
		err := e.Start(addr)
		require.ErrorIs(b, err, http.ErrServerClosed)
	}()

	concurrencyLevels := []int{5, 10, 20, 50}
	for _, clients := range concurrencyLevels {
		b.Run(fmt.Sprintf("%d_clients", clients), func(b *testing.B) {
			var tokens []*oauth2.Token
			for i := 0; i < b.N; i++ {
				tokens = append(tokens, op.GetToken(b))
			}

			b.ResetTimer()

			var wg sync.WaitGroup
			ch := make(chan int, clients)
			for i := 0; i < b.N; i++ {
				token := tokens[i]
				wg.Add(1)
				ch <- 1
				go func() {
					defer wg.Done()
					testHttpRequest(b, urlString, token)
					<-ch
				}()
			}
			wg.Wait()
		})
	}
}

func testHttpRequest(t testing.TB, urlString string, token *oauth2.Token) {
	req, err := http.NewRequest(http.MethodGet, urlString, nil)
	require.NoError(t, err)
	token.SetAuthHeader(req)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer require.NoError(t, res.Body.Close())

	require.Equal(t, http.StatusOK, res.StatusCode)
}

func TestHandlerLazyLoad(t *testing.T) {
	op := server.NewTesting(t)
	defer op.Close(t)

	handler := testGetEchoHandler(t)

	oidcDiscoveryHandler, err := newHandler(Options{
		Issuer:            "http://foo.bar/baz",
		RequiredAudience:  "test-client",
		RequiredTokenType: "JWT+AT",
		LazyLoadJwks:      true,
	})
	require.NoError(t, err)

	e := echo.New()
	h := middleware.JWTWithConfig(middleware.JWTConfig{
		ParseTokenFunc: oidcDiscoveryHandler.parseToken,
	})(handler)

	// Test without authentication
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/", nil)
	recNoAuth := httptest.NewRecorder()
	cNoAuth := e.NewContext(reqNoAuth, recNoAuth)

	err = h(cNoAuth)
	require.Error(t, err)

	// Test with authentication
	token := op.GetToken(t)
	testHandlerWithAuthenticationFailure(t, token, h, e)

	oidcDiscoveryHandler.issuer = op.GetURL(t)
	oidcDiscoveryHandler.discoveryUri = getDiscoveryUriFromIssuer(op.GetURL(t))

	testHandlerWithAuthentication(t, token, h, e)
}

func TestHandlerRequirements(t *testing.T) {
	op := server.NewTesting(t)
	defer op.Close(t)

	handler := testGetEchoHandler(t)

	cases := []struct {
		testDescription string
		options         Options
		succeeds        bool
	}{
		{
			testDescription: "no requirements",
			options: Options{
				Issuer: op.GetURL(t),
			},
			succeeds: true,
		},
		{
			testDescription: "required token type matches",
			options: Options{
				Issuer:            op.GetURL(t),
				RequiredTokenType: "JWT+AT",
			},
			succeeds: true,
		},
		{
			testDescription: "required token type doesn't match",
			options: Options{
				Issuer:            op.GetURL(t),
				RequiredTokenType: "FOO",
			},
			succeeds: false,
		},
		{
			testDescription: "required audience matches",
			options: Options{
				Issuer:           op.GetURL(t),
				RequiredAudience: "test-client",
			},
			succeeds: true,
		},
		{
			testDescription: "required audience doesn't match",
			options: Options{
				Issuer:           op.GetURL(t),
				RequiredAudience: "foo",
			},
			succeeds: false,
		},
		{
			testDescription: "required sub matches",
			options: Options{
				Issuer: op.GetURL(t),
				RequiredClaims: map[string]interface{}{
					"sub": "test",
				},
			},
			succeeds: true,
		},
		{
			testDescription: "required sub doesn't match",
			options: Options{
				Issuer: op.GetURL(t),
				RequiredClaims: map[string]interface{}{
					"sub": "foo",
				},
			},
			succeeds: false,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		e := echo.New()
		h := middleware.JWTWithConfig(middleware.JWTConfig{
			ParseTokenFunc: New(c.options),
		})(handler)

		token := op.GetToken(t)

		if c.succeeds {
			testHandlerWithAuthentication(t, token, h, e)
		} else {
			testHandlerWithAuthenticationFailure(t, token, h, e)
		}
	}
}

func testGetEchoHandler(t testing.TB) func(c echo.Context) error {
	t.Helper()

	return func(c echo.Context) error {
		token, ok := c.Get("user").(jwt.Token)
		if !ok {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
		}

		claims, err := token.AsMap(c.Request().Context())
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
		}

		return c.JSON(http.StatusOK, claims)
	}
}

func testHandlerWithAuthentication(t testing.TB, token *oauth2.Token, restrictedHandler echo.HandlerFunc, e *echo.Echo) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	token.Valid()
	token.SetAuthHeader(req)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := restrictedHandler(c)
	require.NoError(t, err)

	res := rec.Result()

	require.Equal(t, http.StatusOK, res.StatusCode)
}

func testHandlerWithAuthenticationFailure(t testing.TB, token *oauth2.Token, restrictedHandler echo.HandlerFunc, e *echo.Echo) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	token.Valid()
	token.SetAuthHeader(req)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := restrictedHandler(c)
	require.Error(t, err)
}

func testHandlerWithIDTokenFailure(t testing.TB, token *oauth2.Token, restrictedHandler echo.HandlerFunc, e *echo.Echo) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	idToken, ok := token.Extra("id_token").(string)
	require.True(t, ok)

	token.AccessToken = idToken

	token.SetAuthHeader(req)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := restrictedHandler(c)
	require.Error(t, err)
	require.Contains(t, err.Error(), "type \"JWT+AT\" required")
}

func TestNew(t *testing.T) {
	op := server.NewTesting(t)
	defer op.Close(t)

	cases := []struct {
		testDescription string
		config          Options
		expectPanic     bool
	}{
		{
			testDescription: "valid issuer doesn't panic",
			config: Options{
				Issuer: op.GetURL(t),
			},
			expectPanic: false,
		},
		{
			testDescription: "valid issuer, invalid DiscoveryUri panics",
			config: Options{
				Issuer:       op.GetURL(t),
				DiscoveryUri: "http://foo.bar/baz",
			},
			expectPanic: true,
		},
		{
			testDescription: "valid issuer, invalid JwksUri panics",
			config: Options{
				Issuer:  op.GetURL(t),
				JwksUri: "http://foo.bar/baz",
			},
			expectPanic: true,
		},
		{
			testDescription: "empty config panics",
			config:          Options{},
			expectPanic:     true,
		},
		{
			testDescription: "fake issuer panics",
			config: Options{
				Issuer: "http://foo.bar/baz",
			},
			expectPanic: true,
		},
		{
			testDescription: "fake issuer with lazy load doesn't panic",
			config: Options{
				Issuer:       "http://foo.bar/baz",
				LazyLoadJwks: true,
			},
			expectPanic: false,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)
		if c.expectPanic {
			require.Panics(t, func() { New(c.config) })
		} else {
			require.NotPanics(t, func() { New(c.config) })
		}
	}
}

func TestGetHeadersFromTokenString(t *testing.T) {
	key, _ := testNewKey(t)

	// Test with KeyID and Type
	token1 := jwt.New()
	token1.Set("foo", "bar")

	headers1 := jws.NewHeaders()
	headers1.Set(jws.TypeKey, "JWT")

	signedTokenBytes1, err := jwt.Sign(token1, jwa.ES384, key, jwt.WithHeaders(headers1))
	require.NoError(t, err)

	signedToken1 := string(signedTokenBytes1)
	parsedHeaders1, err := getHeadersFromTokenString(signedToken1)
	require.NoError(t, err)

	require.Equal(t, key.KeyID(), parsedHeaders1.KeyID())
	require.Equal(t, headers1.Type(), parsedHeaders1.Type())

	// Test with empty headers
	payload1 := `{"foo":"bar"}`

	headers2 := jws.NewHeaders()

	signedTokenBytes2, err := jws.Sign([]byte(payload1), jwa.ES384, key, jws.WithHeaders(headers2))
	require.NoError(t, err)

	signedToken2 := string(signedTokenBytes2)
	parsedHeaders2, err := getHeadersFromTokenString(signedToken2)
	require.NoError(t, err)

	require.Empty(t, parsedHeaders2.Type())

	// Test with multiple signatures
	payload2 := `{"foo":"bar"}`

	signer1, err := jws.NewSigner(jwa.ES384)
	require.NoError(t, err)
	signer2, err := jws.NewSigner(jwa.ES384)
	require.NoError(t, err)

	signedTokenBytes3, err := jws.SignMulti([]byte(payload2), jws.WithSigner(signer1, key, nil, nil), jws.WithSigner(signer2, key, nil, nil))
	require.NoError(t, err)

	signedToken3 := string(signedTokenBytes3)

	_, err = getHeadersFromTokenString(signedToken3)
	require.Error(t, err)
	require.Equal(t, "more than one signature in token", err.Error())

	// Test with non-token string
	_, err = getHeadersFromTokenString("foo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unable to parse tokenString")
}

func TestGetKeyIDFromTokenString(t *testing.T) {
	key, _ := testNewKey(t)

	// Test with KeyID
	token1 := jwt.New()
	token1.Set("foo", "bar")

	headers1 := jws.NewHeaders()

	signedTokenBytes1, err := jwt.Sign(token1, jwa.ES384, key, jwt.WithHeaders(headers1))
	require.NoError(t, err)

	signedToken1 := string(signedTokenBytes1)
	keyID, err := getKeyIDFromTokenString(signedToken1)
	require.NoError(t, err)

	require.Equal(t, key.KeyID(), keyID)

	// Test without KeyID
	keyWithoutKeyID := key
	err = keyWithoutKeyID.Remove(jwk.KeyIDKey)
	require.NoError(t, err)

	token2 := jwt.New()
	token2.Set("foo", "bar")

	headers2 := jws.NewHeaders()

	signedTokenBytes2, err := jwt.Sign(token2, jwa.ES384, keyWithoutKeyID, jwt.WithHeaders(headers2))
	require.NoError(t, err)

	signedToken2 := string(signedTokenBytes2)
	_, err = getKeyIDFromTokenString(signedToken2)
	require.Error(t, err)
	require.Equal(t, "token header does not contain key id (kid)", err.Error())

	// Test with non-token string
	_, err = getKeyIDFromTokenString("foo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unable to parse tokenString")
}

func TestGetTokenTypeFromTokenString(t *testing.T) {
	key, _ := testNewKey(t)

	// Test with Type
	token1 := jwt.New()
	token1.Set("foo", "bar")

	headers1 := jws.NewHeaders()
	headers1.Set(jws.TypeKey, "foo")

	signedTokenBytes1, err := jwt.Sign(token1, jwa.ES384, key, jwt.WithHeaders(headers1))
	require.NoError(t, err)

	signedToken1 := string(signedTokenBytes1)
	tokenType, err := getTokenTypeFromTokenString(signedToken1)
	require.NoError(t, err)

	require.Equal(t, headers1.Type(), tokenType)

	// Test without KeyID
	payload1 := `{"foo":"bar"}`

	signer1, err := jws.NewSigner(jwa.ES384)
	require.NoError(t, err)

	signedTokenBytes2, err := jws.SignMulti([]byte(payload1), jws.WithSigner(signer1, key, nil, nil))
	require.NoError(t, err)

	signedToken2 := string(signedTokenBytes2)
	_, err = getTokenTypeFromTokenString(signedToken2)
	require.Error(t, err)
	require.Equal(t, "token header does not contain type (typ)", err.Error())

	// Test with non-token string
	_, err = getTokenTypeFromTokenString("foo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unable to parse tokenString")
}

func TestIsTokenAudienceValid(t *testing.T) {
	cases := []struct {
		testDescription  string
		requiredAudience string
		tokenAudiences   []string
		expectedResult   bool
	}{
		{
			testDescription:  "empty requiredAudience, empty tokenAudiences",
			requiredAudience: "",
			tokenAudiences:   []string{},
			expectedResult:   true,
		},
		{
			testDescription:  "empty requiredAudience, one tokenAudiences",
			requiredAudience: "",
			tokenAudiences:   []string{"foo"},
			expectedResult:   true,
		},
		{
			testDescription:  "empty requiredAudience, two tokenAudiences",
			requiredAudience: "",
			tokenAudiences:   []string{"foo", "bar"},
			expectedResult:   true,
		},
		{
			testDescription:  "empty requiredAudience, three tokenAudiences",
			requiredAudience: "",
			tokenAudiences:   []string{"foo", "bar", "baz"},
			expectedResult:   true,
		},
		{
			testDescription:  "one tokenAudiences, same as requiredAudience",
			requiredAudience: "foo",
			tokenAudiences:   []string{"foo"},
			expectedResult:   true,
		},
		{
			testDescription:  "two tokenAudiences, first same as requiredAudience",
			requiredAudience: "foo",
			tokenAudiences:   []string{"foo", "bar"},
			expectedResult:   true,
		},
		{
			testDescription:  "two tokenAudiences, second same as requiredAudience",
			requiredAudience: "bar",
			tokenAudiences:   []string{"foo", "bar"},
			expectedResult:   true,
		},
		{
			testDescription:  "three tokenAudiences, third same as requiredAudience",
			requiredAudience: "baz",
			tokenAudiences:   []string{"foo", "bar", "baz"},
			expectedResult:   true,
		},
		{
			testDescription:  "set requiredAudience, empty tokenAudiences",
			requiredAudience: "foo",
			tokenAudiences:   []string{},
			expectedResult:   false,
		},
		{
			testDescription:  "one tokenAudience, not same as requiredAudience",
			requiredAudience: "foo",
			tokenAudiences:   []string{"bar"},
			expectedResult:   false,
		},
		{
			testDescription:  "two tokenAudience, none same as requiredAudience",
			requiredAudience: "foo",
			tokenAudiences:   []string{"bar", "baz"},
			expectedResult:   false,
		},
		{
			testDescription:  "three tokenAudience, none same as requiredAudience",
			requiredAudience: "foo",
			tokenAudiences:   []string{"bar", "baz", "foobar"},
			expectedResult:   false,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)
		result := isTokenAudienceValid(c.requiredAudience, c.tokenAudiences)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestTokenExpirationValid(t *testing.T) {
	cases := []struct {
		testDescription string
		expiration      time.Time
		allowedDrift    time.Duration
		expectedResult  bool
	}{
		{
			testDescription: "expires now, 50 millisecond drift allowed",
			expiration:      time.Now(),
			allowedDrift:    50 * time.Millisecond,
			expectedResult:  true,
		},
		{
			testDescription: "expires now, 10 second drift allowed",
			expiration:      time.Now(),
			allowedDrift:    10 * time.Second,
			expectedResult:  true,
		},
		{
			testDescription: "expires in one hour, 10 second drift allowed",
			expiration:      time.Now().Add(1 * time.Hour),
			allowedDrift:    10 * time.Second,
			expectedResult:  true,
		},
		{
			testDescription: "expired 5 seconds ago, 10 second drift allowed",
			expiration:      time.Now().Add(-5 * time.Second),
			allowedDrift:    10 * time.Second,
			expectedResult:  true,
		},
		{
			testDescription: "expired 11 seconds ago, 10 second drift allowed",
			expiration:      time.Now().Add(-11 * time.Second),
			allowedDrift:    10 * time.Second,
			expectedResult:  false,
		},
		{
			testDescription: "expires now, no drift",
			expiration:      time.Now(),
			allowedDrift:    0,
			expectedResult:  false,
		},
		{
			testDescription: "expired an hour ago, no drift",
			expiration:      time.Now().Add(-1 * time.Hour),
			allowedDrift:    0,
			expectedResult:  false,
		},
		{
			testDescription: "expired an hour ago, 10 second drift",
			expiration:      time.Now().Add(-1 * time.Hour),
			allowedDrift:    10 * time.Second,
			expectedResult:  false,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)
		result := isTokenExpirationValid(c.expiration, c.allowedDrift)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestIsTokenIssuerValid(t *testing.T) {
	cases := []struct {
		testDescription string
		requiredIssuer  string
		tokenIssuer     string
		expectedResult  bool
	}{
		{
			testDescription: "both requiredIssuer and tokenIssuer are the same",
			requiredIssuer:  "foo",
			tokenIssuer:     "foo",
			expectedResult:  true,
		},
		{
			testDescription: "requiredIssuer and tokenIssuer are not the same",
			requiredIssuer:  "foo",
			tokenIssuer:     "bar",
			expectedResult:  false,
		},
		{
			testDescription: "both requiredIssuer and tokenIssuer are empty",
			requiredIssuer:  "",
			tokenIssuer:     "",
			expectedResult:  false,
		},
		{
			testDescription: "requiredIssuer is empty and tokenIssuer is set",
			requiredIssuer:  "",
			tokenIssuer:     "foo",
			expectedResult:  false,
		},
		{
			testDescription: "requiredIssuer is set and tokenIssuer is empty",
			requiredIssuer:  "foo",
			tokenIssuer:     "",
			expectedResult:  false,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)
		result := isTokenIssuerValid(c.requiredIssuer, c.tokenIssuer)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestIsTokenTypeValid(t *testing.T) {
	cases := []struct {
		testDescription   string
		requiredTokenType string
		tokenType         string
		expectedResult    bool
	}{
		{
			testDescription:   "both requiredTokenType and tokenType are empty",
			requiredTokenType: "",
			tokenType:         "",
			expectedResult:    true,
		},
		{
			testDescription:   "requiredTokenType is empty and tokenType is set",
			requiredTokenType: "",
			tokenType:         "foo",
			expectedResult:    true,
		},
		{
			testDescription:   "both requiredTokenType and tokenType are set to the same",
			requiredTokenType: "foo",
			tokenType:         "foo",
			expectedResult:    true,
		},
		{
			testDescription:   "requiredTokenType and tokenType are set to different",
			requiredTokenType: "foo",
			tokenType:         "bar",
			expectedResult:    false,
		},
		{
			testDescription:   "requiredTokenType and tokenType are set to different but tokenType contains requiredTokenType",
			requiredTokenType: "foo",
			tokenType:         "foobar",
			expectedResult:    false,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		key, _ := testNewKey(t)
		payload := `{"foo":"bar"}`

		signer, err := jws.NewSigner(jwa.ES384)
		require.NoError(t, err)

		var signedTokenBytes []byte
		if c.tokenType == "" {
			signedTokenBytes, err = jws.SignMulti([]byte(payload), jws.WithSigner(signer, key, nil, nil))
			require.NoError(t, err)
		} else {
			headers := jws.NewHeaders()
			headers.Set(jws.TypeKey, c.tokenType)

			signedTokenBytes, err = jws.SignMulti([]byte(payload), jws.WithSigner(signer, key, nil, headers))
			require.NoError(t, err)
		}

		token := string(signedTokenBytes)

		result := isTokenTypeValid(c.requiredTokenType, token)
		require.Equal(t, c.expectedResult, result)
	}
}

func TestGetAndValidateTokenFromString(t *testing.T) {
	op := server.NewTesting(t)
	defer op.Close(t)

	issuer := op.GetURL(t)
	discoveryUri := getDiscoveryUriFromIssuer(issuer)
	jwksUri, err := getJwksUriFromDiscoveryUri(http.DefaultClient, discoveryUri, 10*time.Millisecond)
	require.NoError(t, err)

	keyHandler, err := newKeyHandler(http.DefaultClient, jwksUri, 10*time.Millisecond, 100, false)
	require.NoError(t, err)

	validKey, ok := keyHandler.getKeySet().Get(0)
	require.True(t, ok)

	validAccessToken := op.GetToken(t).AccessToken
	require.NotEmpty(t, validAccessToken)

	validIDToken, ok := op.GetToken(t).Extra("id_token").(string)
	require.True(t, ok)
	require.NotEmpty(t, validIDToken)

	invalidKey, invalidPubKey := testNewKey(t)

	invalidToken := jwt.New()
	invalidToken.Set("foo", "bar")

	invalidHeaders := jws.NewHeaders()
	invalidHeaders.Set(jws.TypeKey, "JWT")

	invalidTokenBytes, err := jwt.Sign(invalidToken, jwa.ES384, invalidKey, jwt.WithHeaders(invalidHeaders))
	require.NoError(t, err)

	invalidSignedToken := string(invalidTokenBytes)

	cases := []struct {
		testDescription string
		tokenString     string
		key             jwk.Key
		expectedError   bool
	}{
		{
			testDescription: "valid access token, valid key",
			tokenString:     validAccessToken,
			key:             validKey,
			expectedError:   false,
		},
		{
			testDescription: "valid id token, valid key",
			tokenString:     validIDToken,
			key:             validKey,
			expectedError:   false,
		},
		{
			testDescription: "empty string, valid key",
			tokenString:     "",
			key:             validKey,
			expectedError:   true,
		},
		{
			testDescription: "random string, valid key",
			tokenString:     "foobar",
			key:             validKey,
			expectedError:   true,
		},
		{
			testDescription: "invalid token, valid key",
			tokenString:     invalidSignedToken,
			key:             validKey,
			expectedError:   true,
		},
		{
			testDescription: "invalid token, invalid key",
			tokenString:     invalidSignedToken,
			key:             invalidPubKey,
			expectedError:   false,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		token, err := getAndValidateTokenFromString(c.tokenString, c.key, false)
		if c.expectedError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.NotEmpty(t, token)
		}
	}
}

func TestParseToken(t *testing.T) {
	keySets := testNewTestKeySet(t)
	testServer := testNewJwksServer(t, keySets)
	defer testServer.Close()

	cases := []struct {
		testDescription         string
		options                 Options
		numKeys                 int
		customIssuer            string
		customExpirationMinutes int
		customClaims            map[string]string
		expectedErrorContains   string
	}{
		{
			testDescription: "successful parse with keyID, one key",
			options: Options{
				Issuer:        "http://foo.bar",
				DiscoveryUri:  "http://foo.bar",
				JwksUri:       testServer.URL,
				DisableKeyID:  false,
				JwksRateLimit: 100,
			},
			numKeys:               1,
			expectedErrorContains: "",
		},
		{
			testDescription: "successful parse without keyID, one key",
			options: Options{
				Issuer:        "http://foo.bar",
				DiscoveryUri:  "http://foo.bar",
				JwksUri:       testServer.URL,
				DisableKeyID:  true,
				JwksRateLimit: 100,
			},
			numKeys:               1,
			expectedErrorContains: "",
		},
		{
			testDescription: "successful parse with keyID, two keys",
			options: Options{
				Issuer:        "http://foo.bar",
				DiscoveryUri:  "http://foo.bar",
				JwksUri:       testServer.URL,
				DisableKeyID:  false,
				JwksRateLimit: 100,
			},
			numKeys:               2,
			expectedErrorContains: "",
		},
		{
			// without lazyLoad, New() panics
			testDescription: "unsuccessful parse without keyID, two keys with lazyLoad",
			options: Options{
				Issuer:        "http://foo.bar",
				DiscoveryUri:  "http://foo.bar",
				JwksUri:       testServer.URL,
				DisableKeyID:  true,
				JwksRateLimit: 100,
				LazyLoadJwks:  true,
			},
			numKeys:               2,
			expectedErrorContains: "keyID is disabled, but received a keySet with more than one key",
		},
		{
			testDescription: "wrong issuer, with keyID",
			options: Options{
				Issuer:       "http://foo.bar",
				DiscoveryUri: "http://foo.bar",
				JwksUri:      testServer.URL,
				DisableKeyID: false,
			},
			numKeys:               1,
			customIssuer:          "http://wrong.issuer",
			expectedErrorContains: "required issuer \"http://foo.bar\" was not found",
		},
		{
			testDescription: "wrong issuer, without keyID",
			options: Options{
				Issuer:       "http://foo.bar",
				DiscoveryUri: "http://foo.bar",
				JwksUri:      testServer.URL,
				DisableKeyID: true,
			},
			numKeys:               1,
			customIssuer:          "http://wrong.issuer",
			expectedErrorContains: "required issuer \"http://foo.bar\" was not found",
		},
		{
			testDescription: "expired token, with keyID",
			options: Options{
				Issuer:       "http://foo.bar",
				DiscoveryUri: "http://foo.bar",
				JwksUri:      testServer.URL,
				DisableKeyID: false,
			},
			numKeys:                 1,
			customExpirationMinutes: -1,
			expectedErrorContains:   "token has expired",
		},
		{
			testDescription: "expired token, without keyID",
			options: Options{
				Issuer:       "http://foo.bar",
				DiscoveryUri: "http://foo.bar",
				JwksUri:      testServer.URL,
				DisableKeyID: true,
			},
			numKeys:                 1,
			customExpirationMinutes: -1,
			expectedErrorContains:   "token has expired",
		},
		{
			testDescription: "correct requiredClaim",
			options: Options{
				Issuer:       "http://foo.bar",
				DiscoveryUri: "http://foo.bar",
				JwksUri:      testServer.URL,
				RequiredClaims: map[string]interface{}{
					"foo": "bar",
				},
				DisableKeyID: false,
			},
			numKeys:               1,
			expectedErrorContains: "",
		},
		{
			testDescription: "correct requiredClaim",
			options: Options{
				Issuer:       "http://foo.bar",
				DiscoveryUri: "http://foo.bar",
				JwksUri:      testServer.URL,
				RequiredClaims: map[string]interface{}{
					"foo": "bar",
				},
				DisableKeyID: false,
			},
			numKeys: 1,
			customClaims: map[string]string{
				"foo": "baz",
			},
			expectedErrorContains: "unable to validate required claims",
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		keySets.setKeys(testNewKeySet(t, c.numKeys, c.options.DisableKeyID))

		parseTokenFunc := New(c.options)

		issuer := c.options.Issuer
		if c.customIssuer != "" {
			issuer = c.customIssuer
		}

		expirationMinutes := 1
		if c.customExpirationMinutes != 0 {
			expirationMinutes = c.customExpirationMinutes
		}

		customClaims := make(map[string]string)
		customClaims["foo"] = "bar"
		if c.customClaims != nil {
			customClaims = c.customClaims
		}

		token := testNewCustomTokenString(t, keySets.privateKeySet, issuer, expirationMinutes, customClaims)

		_, err := parseTokenFunc(token, testNewEchoContext(t))

		if c.expectedErrorContains == "" {
			require.NoError(t, err)
		} else {
			require.Contains(t, err.Error(), c.expectedErrorContains)
		}
	}
}

func TestParseTokenWithKeyID(t *testing.T) {
	disableKeyID := false
	keySets := testNewTestKeySet(t)
	testServer := testNewJwksServer(t, keySets)
	defer testServer.Close()

	keySets.setKeys(testNewKeySet(t, 1, disableKeyID))

	opts := Options{
		Issuer:        "http://foo.bar",
		DiscoveryUri:  "http://foo.bar",
		JwksUri:       testServer.URL,
		DisableKeyID:  disableKeyID,
		JwksRateLimit: 100,
	}

	parseTokenFunc := New(opts)

	// first token should succeed
	token1 := testNewTokenString(t, keySets.privateKeySet)

	_, err := parseTokenFunc(token1, testNewEchoContext(t))
	require.NoError(t, err)

	// second token should succeed, rotation successful
	keySets.setKeys(testNewKeySet(t, 1, disableKeyID))

	token2 := testNewTokenString(t, keySets.privateKeySet)

	_, err = parseTokenFunc(token2, testNewEchoContext(t))
	require.NoError(t, err)

	// after rotation, first token should fail
	_, err = parseTokenFunc(token1, testNewEchoContext(t))
	require.Error(t, err)

	// third token should succeed with two keys
	keySets.setKeys(testNewKeySet(t, 2, disableKeyID))

	token3 := testNewTokenString(t, keySets.privateKeySet)

	_, err = parseTokenFunc(token3, testNewEchoContext(t))
	require.NoError(t, err)

	// fourth token should fail since they token doesn't contain keyID
	keySets.setKeys(testNewKeySet(t, 1, true))

	token4 := testNewTokenString(t, keySets.privateKeySet)

	_, err = parseTokenFunc(token4, testNewEchoContext(t))
	require.Error(t, err)

	// fifth token should fail since it's the wrong key but correct keyID
	keySets.setKeys(testNewKeySet(t, 1, disableKeyID))
	currentPrivateKey, found := keySets.privateKeySet.Get(0)
	require.True(t, found)

	currentKeyID := currentPrivateKey.KeyID()
	invalidPrivKey, _ := testNewKey(t)

	invalidPrivKey.Set(jwk.KeyIDKey, currentKeyID)
	invalidKeySet := jwk.NewSet()
	invalidKeySet.Add(invalidPrivKey)

	token5 := testNewTokenString(t, invalidKeySet)

	_, err = parseTokenFunc(token5, testNewEchoContext(t))
	require.ErrorIs(t, err, errSignatureVerification)

	// sixth token should fail since the jwks can't be refreshed
	keySets.setKeys(testNewKeySet(t, 1, disableKeyID))

	token6 := testNewTokenString(t, keySets.privateKeySet)

	testServer.Close()

	_, err = parseTokenFunc(token6, testNewEchoContext(t))
	require.Error(t, err)
}

func TestParseTokenWithoutKeyID(t *testing.T) {
	disableKeyID := true
	keySets := testNewTestKeySet(t)
	testServer := testNewJwksServer(t, keySets)
	defer testServer.Close()

	keySets.setKeys(testNewKeySet(t, 1, disableKeyID))

	opts := Options{
		Issuer:        "http://foo.bar",
		DiscoveryUri:  "http://foo.bar",
		JwksUri:       testServer.URL,
		DisableKeyID:  disableKeyID,
		JwksRateLimit: 100,
	}

	parseTokenFunc := New(opts)

	// first token should succeed
	token1 := testNewTokenString(t, keySets.privateKeySet)

	_, err := parseTokenFunc(token1, testNewEchoContext(t))
	require.NoError(t, err)

	// second token should succeed, with key rotation
	keySets.setKeys(testNewKeySet(t, 1, disableKeyID))

	token2 := testNewTokenString(t, keySets.privateKeySet)

	_, err = parseTokenFunc(token2, testNewEchoContext(t))
	require.NoError(t, err)

	// after rotation, first token should fail
	_, err = parseTokenFunc(token1, testNewEchoContext(t))
	require.Error(t, err)

	// third token should fail since there are two keys present
	keySets.setKeys(testNewKeySet(t, 2, disableKeyID))

	token3 := testNewTokenString(t, keySets.privateKeySet)

	_, err = parseTokenFunc(token3, testNewEchoContext(t))
	require.Error(t, err)

	// fourth token should fail since the jwks can't be refreshed
	keySets.setKeys(testNewKeySet(t, 1, disableKeyID))

	token4 := testNewTokenString(t, keySets.privateKeySet)

	testServer.Close()

	_, err = parseTokenFunc(token4, testNewEchoContext(t))
	require.Error(t, err)
}

func TestGetAndValidateTokenFromStringWithKeyID(t *testing.T) {
	disableKeyID := false
	keySets := testNewTestKeySet(t)
	testServer := testNewJwksServer(t, keySets)
	defer testServer.Close()

	keySets.setKeys(testNewKeySet(t, 1, disableKeyID))

	keyHandler, err := newKeyHandler(http.DefaultClient, testServer.URL, 10*time.Millisecond, 100, disableKeyID)
	require.NoError(t, err)

	token1 := testNewTokenString(t, keySets.privateKeySet)

	keyID, err := getKeyIDFromTokenString(token1)
	require.NoError(t, err)

	pubKey, err := keyHandler.getKey(context.Background(), keyID)
	require.NoError(t, err)

	_, err = getAndValidateTokenFromString(token1, pubKey, disableKeyID)
	require.NoError(t, err)

	keySets.setKeys(testNewKeySet(t, 1, disableKeyID))

	token2 := testNewTokenString(t, keySets.privateKeySet)

	_, err = getAndValidateTokenFromString(token2, pubKey, disableKeyID)
	require.Error(t, err)
}

func TestGetAndValidateTokenFromStringWithoutKeyID(t *testing.T) {
	disableKeyID := true
	keySets := testNewTestKeySet(t)
	testServer := testNewJwksServer(t, keySets)

	keySets.setKeys(testNewKeySet(t, 1, disableKeyID))

	keyHandler, err := newKeyHandler(http.DefaultClient, testServer.URL, 10*time.Millisecond, 100, disableKeyID)
	require.NoError(t, err)

	token1 := testNewTokenString(t, keySets.privateKeySet)

	pubKey, err := keyHandler.getKey(context.Background(), "")
	require.NoError(t, err)

	_, err = getAndValidateTokenFromString(token1, pubKey, disableKeyID)
	require.NoError(t, err)

	keySets.setKeys(testNewKeySet(t, 1, disableKeyID))

	token2 := testNewTokenString(t, keySets.privateKeySet)

	_, err = getAndValidateTokenFromString(token2, pubKey, disableKeyID)
	require.ErrorIs(t, err, errSignatureVerification)
}

func TestIsRequiredClaimsValid(t *testing.T) {
	cases := []struct {
		testDescription string
		requiredClaims  map[string]interface{}
		tokenClaims     map[string]interface{}
		expectedResult  bool
	}{
		{
			testDescription: "both are nil",
			requiredClaims:  nil,
			tokenClaims:     nil,
			expectedResult:  true,
		},
		{
			testDescription: "both are empty",
			requiredClaims:  map[string]interface{}{},
			tokenClaims:     map[string]interface{}{},
			expectedResult:  true,
		},
		{
			testDescription: "required claims are nil",
			requiredClaims:  nil,
			tokenClaims: map[string]interface{}{
				"foo": "bar",
			},
			expectedResult: true,
		},
		{
			testDescription: "required claims are empty",
			requiredClaims:  map[string]interface{}{},
			tokenClaims: map[string]interface{}{
				"foo": "bar",
			},
			expectedResult: true,
		},
		{
			testDescription: "token claims are nil",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
			},
			tokenClaims:    nil,
			expectedResult: false,
		},
		{
			testDescription: "token claims are empty",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
			},
			tokenClaims:    map[string]interface{}{},
			expectedResult: false,
		},
		{
			testDescription: "required is string, token is int",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
			},
			tokenClaims: map[string]interface{}{
				"foo": 1337,
			},
			expectedResult: false,
		},
		{
			testDescription: "matching with string",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
			},
			tokenClaims: map[string]interface{}{
				"foo": "bar",
			},
			expectedResult: true,
		},
		{
			testDescription: "matching with string and int",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
			},
			tokenClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
			},
			expectedResult: true,
		},
		{
			testDescription: "matching with string and int in different orders",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
			},
			tokenClaims: map[string]interface{}{
				"bar": 1337,
				"foo": "bar",
			},
			expectedResult: true,
		},
		{
			testDescription: "matching with string, int and float",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": 13.37,
			},
			tokenClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": 13.37,
			},
			expectedResult: true,
		},
		{
			testDescription: "not matching with string, int and float",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": 13.37,
			},
			tokenClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": 12.27,
			},
			expectedResult: false,
		},
		{
			testDescription: "matching slice",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": []string{"foo"},
			},
			tokenClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": []string{"foo"},
			},
			expectedResult: true,
		},
		{
			testDescription: "matching slice with multiple values",
			requiredClaims: map[string]interface{}{
				"oof": []string{"foo", "bar"},
			},
			tokenClaims: map[string]interface{}{
				"oof": []string{"foo", "bar", "baz"},
			},
			expectedResult: true,
		},
		{
			testDescription: "required slice contains in token slice",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": []string{"foo"},
			},
			tokenClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": []string{"foo", "bar", "baz"},
			},
			expectedResult: true,
		},
		{
			testDescription: "not matching slice",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": []string{"foo"},
			},
			tokenClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": []string{"bar"},
			},
			expectedResult: false,
		},
		{
			testDescription: "matching map",
			requiredClaims: map[string]interface{}{
				"foo": map[string]string{
					"foo": "bar",
				},
			},
			tokenClaims: map[string]interface{}{
				"foo": map[string]string{
					"foo": "bar",
				},
			},
			expectedResult: true,
		},
		{
			testDescription: "matching map with multiple values",
			requiredClaims: map[string]interface{}{
				"foo": map[string]string{
					"foo": "bar",
					"bar": "foo",
				},
			},
			tokenClaims: map[string]interface{}{
				"foo": map[string]string{
					"a":   "b",
					"foo": "bar",
					"bar": "foo",
					"c":   "d",
				},
			},
			expectedResult: true,
		},
		{
			testDescription: "matching map with multiple keys in token claims",
			requiredClaims: map[string]interface{}{
				"foo": map[string]string{
					"foo": "bar",
				},
			},
			tokenClaims: map[string]interface{}{
				"foo": map[string]string{
					"a":   "b",
					"foo": "bar",
					"c":   "d",
				},
			},
			expectedResult: true,
		},
		{
			testDescription: "not matching map",
			requiredClaims: map[string]interface{}{
				"foo": map[string]string{
					"foo": "bar",
				},
			},
			tokenClaims: map[string]interface{}{
				"foo": map[string]int{
					"foo": 1337,
				},
			},
			expectedResult: false,
		},
		{
			testDescription: "matching map with string slice",
			requiredClaims: map[string]interface{}{
				"foo": map[string][]string{
					"foo": {"bar"},
				},
			},
			tokenClaims: map[string]interface{}{
				"foo": map[string][]string{
					"foo": {"foo", "bar", "baz"},
				},
			},
			expectedResult: true,
		},
		{
			testDescription: "not matching map with string slice",
			requiredClaims: map[string]interface{}{
				"foo": map[string][]string{
					"foo": {"foobar"},
				},
			},
			tokenClaims: map[string]interface{}{
				"foo": map[string][]string{
					"foo": {"foo", "bar", "baz"},
				},
			},
			expectedResult: false,
		},
		{
			testDescription: "matching slice with map",
			requiredClaims: map[string]interface{}{
				"foo": []map[string]string{
					{"bar": "baz"},
				},
			},
			tokenClaims: map[string]interface{}{
				"foo": []map[string]string{
					{"bar": "baz"},
				},
			},
			expectedResult: true,
		},
		{
			testDescription: "not matching slice with map",
			requiredClaims: map[string]interface{}{
				"foo": []map[string]string{
					{"bar": "foobar"},
				},
			},
			tokenClaims: map[string]interface{}{
				"foo": []map[string]string{
					{"bar": "baz"},
				},
			},
			expectedResult: false,
		},
		{
			testDescription: "matching primitive types, slice and map",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": []string{"foo"},
				"oof": []map[string]string{
					{"bar": "baz"},
				},
			},
			tokenClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": []string{"foo"},
				"oof": []map[string]string{
					{"bar": "baz"},
				},
			},
			expectedResult: true,
		},
		{
			testDescription: "matching primitive types, slice and map where token contains multiple values",
			requiredClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": []string{"bar"},
				"oof": []map[string]string{
					{"bar": "baz"},
				},
			},
			tokenClaims: map[string]interface{}{
				"foo": "bar",
				"bar": 1337,
				"baz": []string{"foo", "bar", "baz"},
				"oof": []map[string]string{
					{"a": "b"},
					{"bar": "baz"},
					{"c": "d"},
				},
			},
			expectedResult: true,
		},
	}

	for i, c := range cases {
		t.Logf("Test iteration %d: %s", i, c.testDescription)

		err := isRequiredClaimsValid(c.requiredClaims, c.tokenClaims)

		if c.expectedResult {
			require.NoError(t, err)

		} else {
			require.Error(t, err)
		}
	}
}

func testNewKey(t testing.TB) (jwk.Key, jwk.Key) {
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)

	key, err := jwk.New(ecdsaKey)
	require.NoError(t, err)

	_, ok := key.(jwk.ECDSAPrivateKey)
	require.True(t, ok)

	thumbprint, err := key.Thumbprint(crypto.SHA256)
	require.NoError(t, err)

	keyID := fmt.Sprintf("%x", thumbprint)
	key.Set(jwk.KeyIDKey, keyID)

	pubKey, err := jwk.New(ecdsaKey.PublicKey)
	require.NoError(t, err)

	_, ok = pubKey.(jwk.ECDSAPublicKey)
	require.True(t, ok)

	pubKey.Set(jwk.KeyIDKey, keyID)
	pubKey.Set(jwk.AlgorithmKey, jwa.ES384)

	return key, pubKey
}

func testNewTokenString(t *testing.T, privKeySet jwk.Set) string {
	t.Helper()

	jwtToken := jwt.New()
	jwtToken.Set(jwt.IssuerKey, "http://foo.bar")
	jwtToken.Set(jwt.ExpirationKey, time.Now().Add(1*time.Minute).Unix())
	jwtToken.Set("foo", "bar")

	headers := jws.NewHeaders()
	headers.Set(jws.TypeKey, "JWT")

	privKey, found := privKeySet.Get(0)
	require.True(t, found)

	tokenBytes, err := jwt.Sign(jwtToken, jwa.ES384, privKey, jwt.WithHeaders(headers))
	require.NoError(t, err)

	return string(tokenBytes)
}

func testNewCustomTokenString(t *testing.T, privKeySet jwk.Set, issuer string, expirationMinutes int, customClaims map[string]string) string {
	t.Helper()

	jwtToken := jwt.New()
	jwtToken.Set(jwt.IssuerKey, issuer)
	jwtToken.Set(jwt.ExpirationKey, time.Now().Add(time.Duration(expirationMinutes)*time.Minute).Unix())

	for k, v := range customClaims {
		jwtToken.Set(k, v)
	}

	headers := jws.NewHeaders()
	headers.Set(jws.TypeKey, "JWT")

	privKey, found := privKeySet.Get(0)
	require.True(t, found)

	tokenBytes, err := jwt.Sign(jwtToken, jwa.ES384, privKey, jwt.WithHeaders(headers))
	require.NoError(t, err)

	return string(tokenBytes)
}

func testNewEchoContext(t *testing.T) echo.Context {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec)
}
