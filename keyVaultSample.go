package main

import (
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/arm/keyvault"
	"github.com/Azure/azure-sdk-for-go/arm/resources/resources"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/satori/uuid"
)

var (
	resourceGroupName = "vaultSampleResourceGroup"
	vaultName         = "myvault"
	location          = "westus"
)

func main() {
	err := keyVaultOperations()
	if err != nil {
		fmt.Printf("Error! %s\n", err)
	}
}

func keyVaultOperations() error {
	credentials, err := getCredentials()
	if err != nil {
		return err
	}
	token, err := getToken(credentials)
	if err != nil {
		return err
	}

	groupClient := resources.NewGroupsClient(credentials["AZURE_SUBSCRIPTION_ID"])
	groupClient.Authorizer = token

	vaultsClient := keyvault.NewVaultsClient(credentials["AZURE_SUBSCRIPTION_ID"])
	vaultsClient.Authorizer = token

	fmt.Println("Create resource group...")
	resourceGroupParameters := resources.ResourceGroup{
		Location: &location}
	_, err = groupClient.CreateOrUpdate(resourceGroupName, resourceGroupParameters)
	defer groupClient.Delete(resourceGroupName, nil)
	if err != nil {
		return err
	}

	fmt.Println("Create Key Vault...")
	tenantID, err := uuid.FromString(credentials["AZURE_TENANT_ID"])
	if err != nil {
		return err
	}

	keyVaultParameters := keyvault.VaultCreateOrUpdateParameters{
		Location: &location,
		Properties: &keyvault.VaultProperties{
			TenantID: &tenantID,
			Sku: &keyvault.Sku{
				Family: to.StringPtr("A"),
				Name:   keyvault.Standard},
			AccessPolicies: &[]keyvault.AccessPolicyEntry{
				keyvault.AccessPolicyEntry{
					TenantID: &tenantID,
					ObjectID: &tenantID,
					Permissions: &keyvault.Permissions{
						Keys: &[]keyvault.KeyPermissions{
							keyvault.All},
						Secrets: &[]keyvault.SecretPermissions{
							keyvault.SecretPermissionsAll}},
				},
			}}}

	_, err = vaultsClient.CreateOrUpdate(resourceGroupName, vaultName, keyVaultParameters)
	if err != nil {
		return err
	}

	fmt.Println("Get Key Vault...")
	vault, err := vaultsClient.Get(resourceGroupName, vaultName)
	if err != nil {
		return err
	}
	printKeyVault(vault)

	fmt.Println("List all Key Vaults in subscription...")
	vaultsClient.APIVersion = "2015-11-01"
	// This weird usage of the SDK is caused by this issue https://github.com/Azure/azure-sdk-for-go/issues/403
	sList, err := vaultsClient.List("resourceType eq 'Microsoft.KeyVault/vaults'", nil)
	if err != nil {
		return err
	}
	for _, kv := range *sList.Value {
		printKeyVault(kv)
	}

	fmt.Println("List all Key Vaults in resource group...")
	vaultsClient.APIVersion = keyvault.APIVersion //Get APIVersion back to default
	rgList, err := vaultsClient.ListByResourceGroup(resourceGroupName, nil)
	if err != nil {
		return err
	}
	for _, kv := range *rgList.Value {
		printKeyVault(kv)
	}

	fmt.Println("Delete Key Vault...")
	_, err = vaultsClient.Delete(resourceGroupName, vaultName)
	if err != nil {
		return err
	}

	return nil
}

// getCredentials gets some credentials from your environment variables.
func getCredentials() (map[string]string, error) {
	credentials := map[string]string{
		"AZURE_CLIENT_ID":       os.Getenv("AZURE_CLIENT_ID"),
		"AZURE_CLIENT_SECRET":   os.Getenv("AZURE_CLIENT_SECRET"),
		"AZURE_SUBSCRIPTION_ID": os.Getenv("AZURE_SUBSCRIPTION_ID"),
		"AZURE_TENANT_ID":       os.Getenv("AZURE_TENANT_ID")}
	if err := checkEnvVar(&credentials); err != nil {
		return nil, err
	}
	return credentials, nil
}

// checkEnvVar checks if the environment variables are actually set.
func checkEnvVar(envVars *map[string]string) error {
	var missingVars []string
	for varName, value := range *envVars {
		if value == "" {
			missingVars = append(missingVars, varName)
		}
	}
	if len(missingVars) > 0 {
		return fmt.Errorf("Missing environment variables %s", missingVars)
	}
	return nil
}

// getToken gets a token using your credentials. The token will be used by clients.
func getToken(credentials map[string]string) (*azure.ServicePrincipalToken, error) {
	oauthConfig, err := azure.PublicCloud.OAuthConfigForTenant(credentials["AZURE_TENANT_ID"])
	if err != nil {
		return nil, err
	}
	token, err := azure.NewServicePrincipalToken(*oauthConfig, credentials["AZURE_CLIENT_ID"], credentials["AZURE_CLIENT_SECRET"], azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// printKeyVault prints basic info about a Key Vault.
func printKeyVault(vault keyvault.Vault) {
	tags := "\n"
	if vault.Tags == nil || len(*vault.Tags) <= 0 {
		tags += "\t\tNo tags yet\n"
	} else {
		for k, v := range *vault.Tags {
			tags += fmt.Sprintf("\t\t%s = %s\n", k, *v)
		}
	}
	fmt.Printf("Key vault '%s'\n", *vault.Name)
	elements := map[string]interface{}{
		"ID":       *vault.ID,
		"Type":     *vault.Type,
		"Location": *vault.Location,
		"Tags":     tags}
	for k, v := range elements {
		fmt.Printf("\t%s: %s\n", k, v)
	}
}
