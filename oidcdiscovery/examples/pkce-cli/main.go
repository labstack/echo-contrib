package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cristalhq/aconfig"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/browser"
	"golang.org/x/sync/errgroup"
)

func main() {
	cfg, err := newConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	err = run(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "application returned error: %v\n", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	stopChan := make(chan os.Signal, 2)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGPIPE)

	addr := net.JoinHostPort(cfg.Address, fmt.Sprintf("%d", cfg.Port))
	callbackUrl := fmt.Sprintf("http://%s/callback", addr)
	startUrl := fmt.Sprintf("http://%s/start", addr)

	authzInfo, err := getAuthorizationInfo(callbackUrl, cfg.Issuer, cfg.ClientID, cfg.Scopes)
	if err != nil {
		return err
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Recover())
	e.Use(middleware.Secure())

	e.GET("/start", authzInfo.startHandler)
	e.GET("/callback", authzInfo.callbackHandler)

	g.Go(func() error {
		err := e.Start(addr)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	})

	g.Go(func() error {
		return openUrlWithBrowser(startUrl)
	})

	select {
	case <-stopChan:
	case <-ctx.Done():
	case <-authzInfo.shutdownHttpServerCh:
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	g.Go(func() error {
		return e.Shutdown(shutdownCtx)
	})

	err = g.Wait()
	if err != nil {
		return err
	}

	tokenCtx := context.Background()
	return authzInfo.getAndPrintToken(tokenCtx)
}

type config struct {
	Address  string `flag:"address" env:"ADDRESS" default:"localhost" usage:"address webserver will listen to"`
	Port     int    `flag:"port" env:"PORT" default:"8080" usage:"port webserver will listen to"`
	Issuer   string `flag:"issuer" env:"ISSUER" usage:"the oidc issuer to use for tokens"`
	ClientID string `flag:"client-id" env:"CLIENT_ID" usage:"the client id to use"`
	Scopes   string `flag:"scopes" env:"SCOPES" default:"openid profile" usage:"scopes to request from authorization server / openid provider"`
}

func newConfig() (config, error) {
	var cfg config

	loader := aconfig.LoaderFor(&cfg, aconfig.Config{
		SkipDefaults: false,
		SkipFiles:    true,
		SkipEnv:      false,
		SkipFlags:    false,
		EnvPrefix:    "",
		FlagPrefix:   "",
		Files:        []string{},
		FileDecoders: map[string]aconfig.FileDecoder{},
	})

	err := loader.Load()
	if err != nil {
		return config{}, err
	}

	return cfg, nil
}

func openUrlWithBrowser(s string) error {
	null, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}

	browser.Stderr = null
	browser.Stdout = null

	return browser.OpenURL(s)
}

type authorizationInfo struct {
	authorizationEndpoint string
	tokenEndpoint         string
	clientID              string
	callback              string
	codeVerifier          string
	codeChallange         string
	state                 string
	responseCode          string
	responseState         string
	shutdownHttpServerCh  chan struct{}
}

func (a *authorizationInfo) startHandler(c echo.Context) error {
	setNoCache(c.Response())

	return c.Redirect(http.StatusMovedPermanently, a.authorizationEndpoint)
}

func (a *authorizationInfo) callbackHandler(c echo.Context) error {
	defer close(a.shutdownHttpServerCh)

	setNoCache(c.Response())

	a.responseCode = c.QueryParam("code")
	a.responseState = c.QueryParam("state")

	return c.HTML(http.StatusOK, "Callback recevied. You can close the tab now.")
}

func (a *authorizationInfo) getAndPrintToken(ctx context.Context) error {
	if a.clientID == "" {
		return fmt.Errorf("clientID is empty")
	}
	if a.codeVerifier == "" {
		return fmt.Errorf("codeVerifier is empty")
	}
	if a.responseCode == "" {
		return fmt.Errorf("responseCode is empty")
	}
	if a.callback == "" {
		return fmt.Errorf("callback is empty")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", a.clientID)
	data.Set("code_verifier", a.codeVerifier)
	data.Set("code", a.responseCode)
	data.Set("redirect_uri", a.callback)

	req, err := http.NewRequestWithContext(ctx, "POST", a.tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	err = res.Body.Close()
	if err != nil {
		return err
	}

	fmt.Printf("%s", string(bodyBytes))

	return nil
}

func getAuthorizationInfo(callback string, issuer string, clientID string, scopes string) (authorizationInfo, error) {
	if callback == "" {
		return authorizationInfo{}, fmt.Errorf("callback is empty")
	}
	if issuer == "" {
		return authorizationInfo{}, fmt.Errorf("issuer is empty")
	}
	if clientID == "" {
		return authorizationInfo{}, fmt.Errorf("clientID is empty")
	}
	if scopes == "" {
		return authorizationInfo{}, fmt.Errorf("scopes is empty")
	}

	codeVerifier, codeChallange, err := generateCodeChallengeS256()
	if err != nil {
		return authorizationInfo{}, err
	}

	state, err := generateState()
	if err != nil {
		return authorizationInfo{}, err
	}

	discovery, err := getDiscoveryData(issuer)
	if err != nil {
		return authorizationInfo{}, err
	}

	authzUrl, err := url.Parse(discovery.AuthorizationEndpoint)
	if err != nil {
		return authorizationInfo{}, err
	}

	query := url.Values{}
	query.Add("client_id", clientID)
	query.Add("code_challenge", codeChallange)
	query.Add("code_challenge_method", "S256")
	query.Add("redirect_uri", callback)
	query.Add("response_type", "code")
	query.Add("scope", scopes)
	query.Add("state", state)

	authzUrl.RawQuery = query.Encode()

	return authorizationInfo{
		authorizationEndpoint: authzUrl.String(),
		tokenEndpoint:         discovery.TokenEndpoint,
		clientID:              clientID,
		callback:              callback,
		codeVerifier:          codeVerifier,
		codeChallange:         codeChallange,
		state:                 state,
		shutdownHttpServerCh:  make(chan struct{}),
	}, nil
}

type discoveryData struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

func getDiscoveryData(issuer string) (discoveryData, error) {
	discoveryUri := fmt.Sprintf("%s/.well-known/openid-configuration", strings.TrimSuffix(issuer, "/"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryUri, nil)
	if err != nil {
		return discoveryData{}, err
	}

	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return discoveryData{}, err
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return discoveryData{}, err
	}

	err = res.Body.Close()
	if err != nil {
		return discoveryData{}, err
	}

	var d discoveryData

	err = json.Unmarshal(bodyBytes, &d)
	if err != nil {
		return discoveryData{}, err
	}

	if d.AuthorizationEndpoint == "" {
		return discoveryData{}, fmt.Errorf("AuthorizationEndpoint is empty")
	}

	if d.TokenEndpoint == "" {
		return discoveryData{}, fmt.Errorf("TokenEndpoint is empty")
	}

	return d, nil
}

func generateCodeChallengeS256() (string, string, error) {
	codeVerifier, err := generateRandomString(43)
	if err != nil {
		return "", "", err
	}

	hasher := sha256.New()
	hasher.Write([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(hasher.Sum(nil))

	return codeVerifier, codeChallenge, nil
}

func generateState() (string, error) {
	stateString, err := generateRandomString(32)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write([]byte(stateString))
	state := base64.RawURLEncoding.EncodeToString(hasher.Sum(nil))

	return state, nil
}

func generateRandomString(n int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		ret[i] = letters[num.Int64()]
	}

	return string(ret), nil
}

func setNoCache(res *echo.Response) {
	epoch := time.Unix(0, 0).Format(time.RFC1123)
	res.Header().Set("Expires", epoch)
	res.Header().Set("Cache-Control", "no-cache, private, max-age=0")
	res.Header().Set("Pragma", "no-cache")
}
