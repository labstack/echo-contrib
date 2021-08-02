# Auth0 example

Create an Auth0 account and a native app. Copy the issuer and client id.

## Run web server

```shell
TOKEN_ISSUER="https://<domain>.auth0.com/"
TOKEN_AUDIENCE="Auth0ClientID"
go run ./oidcdiscovery/examples/auth0/main.go --token-issuer ${TOKEN_ISSUER} --token-audience ${TOKEN_AUDIENCE} --port 8081
```

## Test with curl

```shell
ID_TOKEN=$(go run ./oidcdiscovery/examples/pkce-cli/main.go --issuer ${TOKEN_ISSUER} --client-id ${TOKEN_AUDIENCE} | jq -r ".id_token")
curl -s http://localhost:8081 | jq
curl -s -H "Authorization: Bearer ${ID_TOKEN}" http://localhost:8081 | jq
```