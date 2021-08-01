# Azure AD Example

This is a small example of how to use OIDC Discovery together with Azure AD.

## Create Azure AD App

```shell
AZ_APP_NAME="example-api"
AZ_APP_URI="https://api.example.com"
AZ_APP_ID=$(az ad app create --display-name ${AZ_APP_NAME} --identifier-uris ${AZ_APP_URI} --query appId -o tsv)
AZ_APP_OBJECT_ID=$(az ad app show --id ${AZ_APP_ID} --output tsv --query objectId)
AZ_APP_PERMISSION_ID=$(az ad app show --id ${AZ_APP_ID} --output tsv --query "oauth2Permissions[0].id")
az rest --method PATCH --uri "https://graph.microsoft.com/beta/applications/${AZ_APP_OBJECT_ID}" --body '{"api":{"requestedAccessTokenVersion": 2}}'
# Add Azure CLI as allowed client
az rest --method PATCH --uri "https://graph.microsoft.com/beta/applications/${AZ_APP_OBJECT_ID}" --body "{\"api\":{\"preAuthorizedApplications\":[{\"appId\":\"04b07795-8ddb-461a-bbee-02f9e1bf7b46\",\"permissionIds\":[\"${AZ_APP_PERMISSION_ID}\"]}]}}"
``` 

## Run web server

```shell
TENANT_ID=$(az account show -o json | jq -r .tenantId)
TOKEN_ISSUER="https://login.microsoftonline.com/${TENANT_ID}/v2.0"
TOKEN_AUDIENCE=$(az ad app list --identifier-uri "https://api.example.com" | jq -r ".[0].appId")
go run ./oidcdiscovery/examples/azuread/main.go --token-issuer ${TOKEN_ISSUER} --token-audience ${TOKEN_AUDIENCE} --token-tenant-id ${TENANT_ID}
```

## Test with curl

```shell
ACCESS_TOKEN=$(az account get-access-token --resource "https://api.example.com" | jq -r .accessToken)
curl -s http://localhost:8080 | jq
curl -s -H "Authorization: Bearer ${ACCESS_TOKEN}" http://localhost:8080 | jq
```