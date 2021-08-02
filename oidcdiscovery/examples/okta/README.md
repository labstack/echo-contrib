# Okta example

Create an Okta organization and a native app. Copy the issuer and client id.

## Run web server

```shell
TOKEN_ISSUER="OktaIssuer"
CLIENT_ID="OktaClientID"
go run ./oidcdiscovery/examples/okta/main.go --token-issuer ${TOKEN_ISSUER} --client-id ${CLIENT_ID} --port 8081
```

## Test with curl

```shell
ACCESS_TOKEN=$(go run ./oidcdiscovery/examples/pkce-cli/main.go --issuer ${TOKEN_ISSUER} --client-id ${CLIENT_ID} | jq -r ".access_token")
curl -s http://localhost:8081 | jq
curl -s -H "Authorization: Bearer ${ACCESS_TOKEN}" http://localhost:8081 | jq
```