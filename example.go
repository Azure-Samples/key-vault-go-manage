package main

import (
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/arm/keyvault"
	"github.com/Azure/azure-sdk-for-go/arm/resources/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/Azure/go-autorest/autorest/utils"
	"github.com/satori/uuid"
)

const (
	groupName  = "your-azure-sample-group"
	vaultName1 = "golangvault"
	vaultName2 = "golangvault2"
	westus     = "westus"
	eastus     = "eastus"
)

// This example requires that the following environment vars are set:
//
// AZURE_TENANT_ID: contains your Azure Active Directory tenant ID or domain
// AZURE_CLIENT_ID: contains your Azure Active Directory Application Client ID
// AZURE_CLIENT_SECRET: contains your Azure Active Directory Application Secret
// AZURE_SUBSCRIPTION_ID: contains your Azure Subscription ID
//

var (
	subscriptionID string
	tenantID       string
	clientID       string
	groupClient    resources.GroupsClient
	vaultsClient   keyvault.VaultsClient
	resourceClient resources.GroupClient
)

func init() {
	authorizer, err := utils.GetAuthorizer(azure.PublicCloud)
	onErrorFail(err, "GetAuthorizer failed")

	subscriptionID = utils.GetEnvVarOrExit("AZURE_SUBSCRIPTION_ID")
	tenantID = utils.GetEnvVarOrExit("AZURE_TENANT_ID")
	clientID = utils.GetEnvVarOrExit("AZURE_CLIENT_ID")
	createClients(subscriptionID, authorizer)
}

func main() {
	fmt.Println("Creating resource group")
	resourceGroupParameters := resources.Group{
		Location: to.StringPtr(westus),
	}
	_, err := groupClient.CreateOrUpdate(groupName, resourceGroupParameters)
	onErrorFail(err, "CreateOrUpdate failed")

	fmt.Println("Creating Key Vault")
	tenantIDuuid, err := uuid.FromString(tenantID)
	onErrorFail(err, "Creating a UUID FromString for tenant ID failed")

	keyVaultParameters := keyvault.VaultCreateOrUpdateParameters{
		Location: to.StringPtr(westus),
		Properties: &keyvault.VaultProperties{
			TenantID: &tenantIDuuid,
			Sku: &keyvault.Sku{
				Family: to.StringPtr("A"),
				Name:   keyvault.Standard,
			},
			AccessPolicies: &[]keyvault.AccessPolicyEntry{},
		},
	}
	_, err = vaultsClient.CreateOrUpdate(groupName, vaultName1, keyVaultParameters)
	onErrorFail(err, "CreateOrUpdate failed")

	fmt.Println("Getting Key Vault")
	vault, err := vaultsClient.Get(groupName, vaultName1)
	onErrorFail(err, "Get failed")
	printKeyVault(vault)

	fmt.Println("Authorizing the application associated with the current service principal")
	clientIDuuid, err := uuid.FromString(clientID)
	onErrorFail(err, "Creating a UUID FromString for client ID failed")
	keyVaultParameters.Properties.TenantID = &clientIDuuid
	policy := keyvault.AccessPolicyEntry{
		ObjectID: &clientID,
		TenantID: &clientIDuuid,
		Permissions: &keyvault.Permissions{
			Keys: &[]keyvault.KeyPermissions{
				keyvault.KeyPermissionsAll,
			},
			Secrets: &[]keyvault.SecretPermissions{
				keyvault.SecretPermissionsGet,
				keyvault.SecretPermissionsList,
			},
		},
	}
	keyVaultParameters.Properties.AccessPolicies = &[]keyvault.AccessPolicyEntry{
		policy,
	}
	vault, err = vaultsClient.CreateOrUpdate(groupName, vaultName1, keyVaultParameters)
	onErrorFail(err, "CreateOrUpdate failed")
	printKeyVault(vault)

	fmt.Println("Update a key vault to enable deployments and add permissions to the application")
	keyVaultParameters.Properties.EnabledForDeployment = to.BoolPtr(true)
	keyVaultParameters.Properties.EnabledForTemplateDeployment = to.BoolPtr(true)
	(*keyVaultParameters.Properties.AccessPolicies)[0].Permissions.Secrets = &[]keyvault.SecretPermissions{
		keyvault.SecretPermissionsAll,
	}
	vault, err = vaultsClient.CreateOrUpdate(groupName, vaultName1, keyVaultParameters)
	onErrorFail(err, "CreateOrUpdate failed")
	printKeyVault(vault)

	fmt.Println("Creating another Key Vault")
	vault2, err := vaultsClient.CreateOrUpdate(groupName, vaultName2, keyvault.VaultCreateOrUpdateParameters{
		Location: to.StringPtr(eastus),
		Properties: &keyvault.VaultProperties{
			TenantID: &clientIDuuid,
			Sku: &keyvault.Sku{
				Family: to.StringPtr("A"),
				Name:   keyvault.Standard,
			},
			AccessPolicies: &[]keyvault.AccessPolicyEntry{
				{
					ObjectID: &clientID,
					TenantID: &clientIDuuid,
					Permissions: &keyvault.Permissions{
						Keys: &[]keyvault.KeyPermissions{
							keyvault.KeyPermissionsList,
							keyvault.KeyPermissionsGet,
							keyvault.KeyPermissionsDecrypt,
						},
						Secrets: &[]keyvault.SecretPermissions{
							keyvault.SecretPermissionsGet,
						},
					},
				},
			},
		},
	})
	onErrorFail(err, "CreateOrUpdate failed")
	printKeyVault(vault2)

	fmt.Println("List all Key Vaults in subscription")

	sList, err := resourceClient.List("resourceType eq 'Microsoft.KeyVault/vaults'", "", nil)
	onErrorFail(err, "List failed")
	for _, kv := range *sList.Value {
		fmt.Printf("\t%s\n", *kv.Name)
	}

	fmt.Println("List all Key Vaults in resource group")
	rgList, err := vaultsClient.ListByResourceGroup(groupName, nil)
	onErrorFail(err, "ListByResourceGroup failed")
	for _, kv := range *rgList.Value {
		fmt.Printf("\t%s\n", *kv.Name)
	}

	fmt.Print("Press enter to delete the Key Vaults...")

	var input string
	fmt.Scanln(&input)

	fmt.Println("Deleting Key Vaults")
	_, err = vaultsClient.Delete(groupName, vaultName1)
	onErrorFail(err, fmt.Sprintf("Delete '%s' failed", vaultName1))
	_, err = vaultsClient.Delete(groupName, vaultName2)
	onErrorFail(err, fmt.Sprintf("Delete '%s' failed", vaultName2))

	fmt.Println("Deleting resource group")
	_, errChan := groupClient.Delete(groupName, nil)
	onErrorFail(<-errChan, "Delete failed")
}

// printKeyVault prints basic info about a Key Vault.
func printKeyVault(vault keyvault.Vault) {
	tags := "\n"
	if vault.Tags == nil || len(*vault.Tags) <= 0 {
		tags += "\t\tNo tags yet"
	} else {
		for k, v := range *vault.Tags {
			tags += fmt.Sprintf("\t\t%s = %s\n", k, *v)
		}
	}

	accessPolicies := "\n"
	if vault.Properties.AccessPolicies == nil || len(*vault.Properties.AccessPolicies) <= 0 {
		accessPolicies += "\t\tNo access policies defined"
	} else {
		elements := map[string]interface{}{
			"ObjectID":           *(*vault.Properties.AccessPolicies)[0].ObjectID,
			"Key Permissions":    *(*vault.Properties.AccessPolicies)[0].Permissions.Keys,
			"Secret permissions": *(*vault.Properties.AccessPolicies)[0].Permissions.Secrets,
		}
		for k, v := range elements {
			accessPolicies += fmt.Sprintf("\t\t%s: %s\n", k, v)
		}
	}

	fmt.Printf("Key vault '%s'\n", *vault.Name)
	elements := map[string]interface{}{
		"Location":        *vault.Location,
		"ID":              *vault.ID,
		"Tags":            tags,
		"Sku":             fmt.Sprintf("%s - %s", vault.Properties.Sku.Name, *vault.Properties.Sku.Family),
		"Access Policies": accessPolicies,
	}
	for k, v := range elements {
		fmt.Printf("\t%s: %s\n", k, v)
	}
}

func createClients(subscriptionID string, authorizer *autorest.BearerAuthorizer) {
	sampleUA := fmt.Sprintf("sample/0008/%s", utils.GetCommit())

	groupClient = resources.NewGroupsClient(subscriptionID)
	groupClient.Authorizer = authorizer
	groupClient.Client.AddToUserAgent(sampleUA)

	resourceClient = resources.NewGroupClient(subscriptionID)
	resourceClient.Authorizer = authorizer
	resourceClient.Client.AddToUserAgent(sampleUA)

	vaultsClient = keyvault.NewVaultsClient(subscriptionID)
	vaultsClient.Authorizer = authorizer
	vaultsClient.Client.AddToUserAgent(sampleUA)
}

// onErrorFail prints a failure message and exits the program if err is not nil.
func onErrorFail(err error, message string) {
	if err != nil {
		fmt.Printf("%s: %s", message, err)
		os.Exit(1)
	}
}
