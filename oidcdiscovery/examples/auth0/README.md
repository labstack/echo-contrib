# Auth0 example

Create an Auth0 account and an api as well as a native app.

## Run web server

```shell
TOKEN_ISSUER="https://<domain>.auth0.com/"
TOKEN_AUDIENCE="https://localhost:8081"
CLIENT_ID="Auth0NativeAppClientID"
go run ./oidcdiscovery/examples/auth0/main.go --token-issuer ${TOKEN_ISSUER} --token-audience ${TOKEN_AUDIENCE} --client-id ${CLIENT_ID} --port 8081
```

## Test with curl

```shell
ACCESS_TOKEN=$(go run ./oidcdiscovery/examples/pkce-cli/main.go --issuer ${TOKEN_ISSUER} --client-id ${CLIENT_ID} --extra-authz-params audience:${TOKEN_AUDIENCE} | jq -r ".access_token")
curl -s http://localhost:8081 | jq
curl -s -H "Authorization: Bearer ${ACCESS_TOKEN}" http://localhost:8081 | jq
```