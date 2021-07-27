package oidcdiscovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"
)

// Options defines the options for OIDC Discovery for the `ParseTokenFunc` for the `JWT` middleware
type Options struct {
	// Issuer is the authority that issues the tokens
	Issuer string

	// DiscoveryUri is where the `jwks_uri` will be grabbed
	// Defaults to `fmt.Sprintf("%s/.well-known/openid-configuration", strings.TrimSuffix(issuer, "/"))`
	DiscoveryUri string

	// JwksUri is used to download the public key(s)
	// Defaults to the `jwks_uri` from the response of DiscoveryUri
	JwksUri string

	// JwksFetchTimeout sets the context timeout when downloading the jwks
	// Defaults to 5 seconds
	JwksFetchTimeout time.Duration

	// AllowedTokenDrift adds the duration to the token expiration to allow
	// for time drift between parties.
	// Defaults to 10 seconds
	AllowedTokenDrift time.Duration

	// LazyLoadJwks makes it possible to use OIDC Discovery without being
	// able to load the keys at startup.
	// Default setting is disabled.
	// Please observe: If enabled, it will always load even though settings
	// may be wrong / not working.
	LazyLoadJwks bool

	// RequiredTokenType is used if only specific tokens should be allowed.
	// Default is empty string `""` and means all token types are allowed.
	// Use case could be to configure this if the TokenType (set in the header of the JWT)
	// should be `JWT` or maybe even `JWT+AT` to diffirentiate between access tokens and
	// id tokens. Not all providers support or use this.
	RequiredTokenType string

	// RequiredAudience is used to require a specific Audience `aud` in the claims.
	// Defaults to empty string `""` and means all audiences are allowed.
	RequiredAudience string

	// RequiredClaims is used to require specific claims in the token
	// Defaults to empty map (nil) and won't check for anything else
	// Works with primitive types, slices and maps.
	// Please observe: slices and strings checks that the token contains it, but more is allowed.
	// Required claim []string{"bar"} matches token []string{"foo", "bar", "baz"}
	// Required claim map[string]string{{"foo": "bar"}} matches token map[string]string{{"a": "b"},{"foo": "bar"},{"c": "d"}}
	//
	// Example:
	//
	// ```go
	// map[string]interface{}{
	// 	"foo": "bar",
	// 	"bar": 1337,
	// 	"baz": []string{"bar"},
	// 	"oof": []map[string]string{
	// 		{"bar": "baz"},
	// 	},
	// },
	// ```
	RequiredClaims map[string]interface{}
}

// New returns an OpenID Connect (OIDC) discovery `ParseTokenFunc` to be used
// with the `JWT` middleware.
// See: https://openid.net/connect/
func New(opts Options) func(auth string, c echo.Context) (interface{}, error) {
	h := newHandler(opts)

	return h.parseToken
}

type handler struct {
	issuer            string
	discoveryUri      string
	jwksUri           string
	jwksFetchTimeout  time.Duration
	allowedTokenDrift time.Duration
	requiredAudience  string
	requiredTokenType string
	requiredClaims    map[string]interface{}

	keyHandler *keyHandler
}

func newHandler(opts Options) *handler {
	h := &handler{
		issuer:            opts.Issuer,
		discoveryUri:      opts.DiscoveryUri,
		jwksUri:           opts.JwksUri,
		jwksFetchTimeout:  opts.JwksFetchTimeout,
		allowedTokenDrift: opts.AllowedTokenDrift,
		requiredTokenType: opts.RequiredTokenType,
		requiredAudience:  opts.RequiredAudience,
		requiredClaims:    opts.RequiredClaims,
	}

	err := h.loadJwks()
	if err != nil {
		if !opts.LazyLoadJwks {
			panic(fmt.Sprintf("echo: oidc discovery unable to load jwks: %v", err))
		} else {
			fmt.Fprintf(os.Stderr, "echo: oidc discovery unable to load jwks: %v", err)
		}
	}

	return h
}

func (h *handler) loadJwks() error {
	if h.issuer == "" {
		return fmt.Errorf("issuer is empty")
	}
	if h.discoveryUri == "" {
		h.discoveryUri = getDiscoveryUriFromIssuer(h.issuer)
	}
	if h.jwksUri == "" {
		jwksUri, err := getJwksUriFromDiscoveryUri(h.discoveryUri, 5*time.Second)
		if err != nil {
			return fmt.Errorf("unable to fetch jwksUri from discoveryUri (%s): %w", h.discoveryUri, err)
		}
		h.jwksUri = jwksUri
	}
	if h.jwksFetchTimeout == 0 {
		h.jwksFetchTimeout = 5 * time.Second
	}
	if h.allowedTokenDrift == 0 {
		h.allowedTokenDrift = 10 * time.Second
	}

	keyHandler, err := newKeyHandler(h.jwksUri, h.jwksFetchTimeout)
	if err != nil {
		return fmt.Errorf("unable to initialize keyHandler: %w", err)
	}

	h.keyHandler = keyHandler

	return nil
}

func (h *handler) parseToken(auth string, c echo.Context) (interface{}, error) {
	if h.keyHandler == nil {
		err := h.loadJwks()
		if err != nil {
			return nil, fmt.Errorf("unable to load jwks: %w", err)
		}
	}

	keyID, err := getKeyIDFromTokenString(auth)
	if err != nil {
		return nil, err
	}

	tokenTypeValid := isTokenTypeValid(h.requiredTokenType, auth)
	if !tokenTypeValid {
		return nil, fmt.Errorf("token type %q required", h.requiredTokenType)
	}

	key, err := h.keyHandler.getByKeyID(keyID, false)
	if err != nil {
		return nil, err
	}

	token, err := getAndValidateTokenFromString(auth, key)
	if err != nil {
		return nil, err
	}

	validExpiration := isTokenExpirationValid(token.Expiration(), h.allowedTokenDrift)
	if !validExpiration {
		return nil, fmt.Errorf("token has expired: %s", token.Expiration())
	}

	validIssuer := isTokenIssuerValid(h.issuer, token.Issuer())
	if !validIssuer {
		return nil, fmt.Errorf("required issuer %q was not found, received: %s", h.issuer, token.Issuer())
	}

	validAudience := isTokenAudienceValid(h.requiredAudience, token.Audience())
	if !validAudience {
		return nil, fmt.Errorf("required audience %q was not found, received: %v", h.requiredAudience, token.Audience())
	}

	if h.requiredClaims != nil {
		tokenClaims, err := token.AsMap(c.Request().Context())
		if err != nil {
			return nil, fmt.Errorf("unable to get token claims: %w", err)
		}

		err = isRequiredClaimsValid(h.requiredClaims, tokenClaims)
		if err != nil {
			return nil, fmt.Errorf("unable to validate required claims: %w", err)
		}
	}

	return token, nil
}

type keyHandler struct {
	sync.RWMutex
	jwksURI      string
	keySet       jwk.Set
	fetchTimeout time.Duration
}

func newKeyHandler(jwksUri string, fetchTimeout time.Duration) (*keyHandler, error) {
	h := &keyHandler{
		jwksURI:      jwksUri,
		fetchTimeout: fetchTimeout,
	}

	err := h.updateKeySet()
	if err != nil {
		return nil, err
	}

	return h, nil
}

func (h *keyHandler) updateKeySet() error {
	ctx, cancel := context.WithTimeout(context.Background(), h.fetchTimeout)
	defer cancel()
	keySet, err := jwk.Fetch(ctx, h.jwksURI)
	if err != nil {
		return fmt.Errorf("unable to fetch keys from %q: %v", h.jwksURI, err)
	}

	h.Lock()
	h.keySet = keySet
	h.Unlock()

	return nil
}

func (h *keyHandler) getKeySet() jwk.Set {
	h.RLock()
	defer h.RUnlock()
	return h.keySet
}

func (h *keyHandler) getByKeyID(keyID string, retry bool) (jwk.Key, error) {
	keySet := h.getKeySet()
	key, found := keySet.LookupKeyID(keyID)

	if !found && !retry {
		err := h.updateKeySet()
		if err != nil {
			return nil, fmt.Errorf("unable to update key set for key %q: %v", keyID, err)
		}

		return h.getByKeyID(keyID, true)
	}

	if !found && retry {
		return nil, fmt.Errorf("unable to find key %q", keyID)
	}

	return key, nil
}

func getDiscoveryUriFromIssuer(issuer string) string {
	return fmt.Sprintf("%s/.well-known/openid-configuration", strings.TrimSuffix(issuer, "/"))
}

func getJwksUriFromDiscoveryUri(discoveryUri string, fetchTimeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryUri, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	err = res.Body.Close()
	if err != nil {
		return "", err
	}

	var discoveryData struct {
		JwksUri string `json:"jwks_uri"`
	}

	err = json.Unmarshal(bodyBytes, &discoveryData)
	if err != nil {
		return "", err
	}

	if discoveryData.JwksUri == "" {
		return "", fmt.Errorf("JwksUri is empty")
	}

	return discoveryData.JwksUri, nil
}

func getKeyIDFromTokenString(tokenString string) (string, error) {
	headers, err := getHeadersFromTokenString(tokenString)
	if err != nil {
		return "", err
	}

	keyID := headers.KeyID()
	if keyID == "" {
		return "", fmt.Errorf("token header does not contain key id (kid)")
	}

	return keyID, nil
}

func getTokenTypeFromTokenString(tokenString string) (string, error) {
	headers, err := getHeadersFromTokenString(tokenString)
	if err != nil {
		return "", err
	}

	tokenType := headers.Type()
	if tokenType == "" {
		return "", fmt.Errorf("token header does not contain type (typ)")
	}

	return tokenType, nil
}

func getHeadersFromTokenString(tokenString string) (jws.Headers, error) {
	msg, err := jws.ParseString(tokenString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse tokenString: %w", err)
	}

	signatures := msg.Signatures()
	if len(signatures) != 1 {
		return nil, fmt.Errorf("more than one signature in token")
	}

	headers := signatures[0].ProtectedHeaders()

	return headers, nil
}

func isTokenAudienceValid(requiredAudience string, audiences []string) bool {
	if requiredAudience == "" {
		return true
	}

	for _, audience := range audiences {
		if audience == requiredAudience {
			return true
		}
	}

	return false
}

func isTokenExpirationValid(expiration time.Time, allowedDrift time.Duration) bool {
	expirationWithAllowedDrift := expiration.Round(0).Add(allowedDrift)

	return expirationWithAllowedDrift.After(time.Now())
}

func isTokenIssuerValid(requiredIssuer string, tokenIssuer string) bool {
	if requiredIssuer == "" {
		return false
	}

	return tokenIssuer == requiredIssuer
}

func isTokenTypeValid(requiredTokenType string, tokenString string) bool {
	if requiredTokenType == "" {
		return true
	}

	tokenType, err := getTokenTypeFromTokenString(tokenString)
	if err != nil {
		return false
	}

	if tokenType != requiredTokenType {
		return false
	}

	return true
}

func isRequiredClaimsValid(requiredClaims map[string]interface{}, tokenClaims map[string]interface{}) error {
	for requiredKey, requiredValue := range requiredClaims {
		tokenValue, ok := tokenClaims[requiredKey]
		if !ok {
			return fmt.Errorf("token does not have the claim: %s", requiredKey)
		}

		required, received, err := getCtyValues(requiredValue, tokenValue)
		if err != nil {
			return err
		}

		err = isCtyValueValid(required, received)
		if err != nil {
			return fmt.Errorf("claim %q not valid: %w", requiredKey, err)
		}
	}

	return nil
}

func getAndValidateTokenFromString(tokenString string, key jwk.Key) (jwt.Token, error) {
	keySet := getKeySetFromKey(key)

	token, err := jwt.ParseString(tokenString, jwt.WithKeySet(keySet))
	if err != nil {
		return nil, err
	}

	return token, nil
}

func isSameType(a, b interface{}) bool {
	return fmt.Sprintf("%T", a) == fmt.Sprintf("%T", b)
}

func getKeySetFromKey(key jwk.Key) jwk.Set {
	keySet := jwk.NewSet()
	keySet.Add(key)

	return keySet
}
