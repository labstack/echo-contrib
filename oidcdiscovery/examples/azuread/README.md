# Azure AD Example

This is an example of how to use OIDC Discovery together with Azure AD.

## Create Azure AD Apps

### PKCE-CLI

```shell
AZ_APP_PKCECLI_NAME="pkce-cli"
AZ_APP_PKCECLI_REPLY_URL="http://localhost:8080/callback"
AZ_APP_PKCECLI_ID=$(az ad app create --display-name ${AZ_APP_PKCECLI_NAME} --native-app --reply-urls ${AZ_APP_PKCECLI_REPLY_URL} --query appId -o tsv)
AZ_APP_PKCECLI_OBJECT_ID=$(az ad app show --id ${AZ_APP_PKCECLI_ID} --output tsv --query objectId)
az rest --method PATCH --uri "https://graph.microsoft.com/beta/applications/${AZ_APP_PKCECLI_OBJECT_ID}" --body '{"api":{"requestedAccessTokenVersion": 2}}'
```

### API

```shell
AZ_APP_NAME="example-api"
AZ_APP_URI="https://localhost:8081"
AZ_APP_ID=$(az ad app create --display-name ${AZ_APP_NAME} --identifier-uris ${AZ_APP_URI} --query appId -o tsv)
AZ_APP_OBJECT_ID=$(az ad app show --id ${AZ_APP_ID} --output tsv --query objectId)
AZ_APP_PERMISSION_ID=$(az ad app show --id ${AZ_APP_ID} --output tsv --query "oauth2Permissions[0].id")
az rest --method PATCH --uri "https://graph.microsoft.com/beta/applications/${AZ_APP_OBJECT_ID}" --body '{"api":{"requestedAccessTokenVersion": 2}}'
# Add Azure CLI as allowed client
az rest --method PATCH --uri "https://graph.microsoft.com/beta/applications/${AZ_APP_OBJECT_ID}" --body "{\"api\":{\"preAuthorizedApplications\":[{\"appId\":\"04b07795-8ddb-461a-bbee-02f9e1bf7b46\",\"permissionIds\":[\"${AZ_APP_PERMISSION_ID}\"]}]}}"
# Add PKCE-CLI as allowed client
az rest --method PATCH --uri "https://graph.microsoft.com/beta/applications/${AZ_APP_OBJECT_ID}" --body "{\"api\":{\"preAuthorizedApplications\":[{\"appId\":\"04b07795-8ddb-461a-bbee-02f9e1bf7b46\",\"permissionIds\":[\"${AZ_APP_PERMISSION_ID}\"]},{\"appId\":\"${AZ_APP_PKCECLI_ID}\",\"permissionIds\":[\"${AZ_APP_PERMISSION_ID}\"]}]}}"
``` 

## Run web server

```shell
TENANT_ID=$(az account show -o json | jq -r .tenantId)
TOKEN_ISSUER="https://login.microsoftonline.com/${TENANT_ID}/v2.0"
TOKEN_AUDIENCE=$(az ad app list --identifier-uri ${AZ_APP_URI} | jq -r ".[0].appId")
go run ./oidcdiscovery/examples/azuread/main.go --token-issuer ${TOKEN_ISSUER} --token-audience ${TOKEN_AUDIENCE} --token-tenant-id ${TENANT_ID} --port 8081
```

## Test with curl

### Using Azure CLI

```shell
ACCESS_TOKEN=$(az account get-access-token --resource ${AZ_APP_URI} | jq -r .accessToken)
curl -s http://localhost:8081 | jq
curl -s -H "Authorization: Bearer ${ACCESS_TOKEN}" http://localhost:8081 | jq
```

### Using PKCE-CLI

```shell
ACCESS_TOKEN=$(go run ./oidcdiscovery/examples/pkce-cli/main.go --issuer ${TOKEN_ISSUER} --client-id ${AZ_APP_PKCECLI_ID} --scopes "openid profile ${AZ_APP_URI}/user_impersonation" | jq -r ".access_token")
curl -s http://localhost:8081 | jq
curl -s -H "Authorization: Bearer ${ACCESS_TOKEN}" http://localhost:8081 | jq
```
