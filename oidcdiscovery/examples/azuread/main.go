package main

import (
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/cristalhq/aconfig"
	"github.com/labstack/echo-contrib/oidcdiscovery"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/lestrrat-go/jwx/jwt"
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
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.Secure())

	e.Use(middleware.JWTWithConfig(middleware.JWTConfig{
		ParseTokenFunc: oidcdiscovery.New(oidcdiscovery.Options{
			Issuer:                     cfg.Issuer,
			RequiredTokenType:          "JWT",
			RequiredAudience:           cfg.Audience,
			FallbackSignatureAlgorithm: cfg.FallbackSignatureAlgorithm,
			RequiredClaims: map[string]interface{}{
				"tid": cfg.TenantID,
			},
		}),
	}))

	e.GET("/", getClaimsHandler)

	addr := net.JoinHostPort(cfg.Address, fmt.Sprintf("%d", cfg.Port))
	return e.Start(addr)
}

func getClaimsHandler(c echo.Context) error {
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

type config struct {
	Address                    string `flag:"address" env:"ADDRESS" default:"127.0.0.1" usage:"address webserver will listen to"`
	Port                       int    `flag:"port" env:"PORT" default:"8080" usage:"port webserver will listen to"`
	Issuer                     string `flag:"token-issuer" env:"TOKEN_ISSUER" usage:"the oidc issuer url for tokens"`
	Audience                   string `flag:"token-audience" env:"TOKEN_AUDIENCE" usage:"the audience that tokens need to contain"`
	TenantID                   string `flag:"token-tenant-id" env:"TOKEN_TENANT_ID" usage:"the tenant id (tid) that tokens need to contain"`
	FallbackSignatureAlgorithm string `flag:"fallback-signature-algorithm" env:"FALLBACK_SIGNATURE_ALGORITHM" default:"RS256" usage:"if the issue jwks doesn't contain key alg, use the following signature algorithm to verify the signature of the tokens"`
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
