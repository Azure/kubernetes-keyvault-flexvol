// Copyright (c) Microsoft and contributors.  All rights reserved.
//
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

const (
)

var (
	oauthConfig	*adal.OAuthConfig
)

// OAuthGrantType specifies which grant type to use.
type OAuthGrantType int

const (
	// OAuthGrantTypeServicePrincipal for client credentials flow
	OAuthGrantTypeServicePrincipal OAuthGrantType = iota
	// OAuthGrantTypeDeviceFlow for device-auth flow
	OAuthGrantTypeDeviceFlow
)

// AzureAuthConfig holds auth related part of cloud config
type AzureAuthConfig struct {
	// The cloud environment identifier. Takes values from https://github.com/Azure/go-autorest/blob/ec5f4903f77ed9927ac95b19ab8e44ada64c1356/autorest/azure/environments.go#L13
	Cloud string `json:"cloud"`
	// The AAD Tenant ID for the Subscription that the cluster is deployed in
	TenantID string `json:"tenantId"`
	// The ClientID for an AAD application with RBAC access to talk to Azure RM APIs
	AADClientID string `json:"aadClientId"`
	// The ClientSecret for an AAD application with RBAC access to talk to Azure RM APIs
	AADClientSecret string `json:"aadClientSecret"`
	// The path of a client certificate for an AAD application with RBAC access to talk to Azure RM APIs
	AADClientCertPath string `json:"aadClientCertPath"`
	// The password of the client certificate for an AAD application with RBAC access to talk to Azure RM APIs
	AADClientCertPassword string `json:"aadClientCertPassword"`
	// Use managed service identity for the virtual machine to access Azure ARM APIs
	UseManagedIdentityExtension bool `json:"useManagedIdentityExtension"`
	// The ID of the Azure Subscription that the cluster is deployed in
	SubscriptionID string `json:"subscriptionId"`
}

// Config holds the configuration parsed from the --cloud-config flag
// All fields are required unless otherwise specified
type Config struct {
	AzureAuthConfig
	// Resource Group for cluster
	ResourceGroup string `json:"resourceGroup"`
	// The kms provider vault name
	ProviderVaultName string `json:"providerVaultName"`
	// The kms provider key name
	ProviderKeyName string `json:"providerKeyName"`
	// The kms provider key version
	ProviderKeyVersion string `json:"providerKeyVersion"`
}

func AuthGrantType() OAuthGrantType {
	return OAuthGrantTypeServicePrincipal
}

func GetManagementToken(grantType OAuthGrantType, cloudName string, tenantId string, useManagedIdentityExtension bool, aADClientSecret string, aADClientID string) (authorizer autorest.Authorizer, err error) {
	
	env, err := ParseAzureEnvironment(cloudName)
	if err != nil {
		return nil, err
	}

	rmEndPoint := env.ResourceManagerEndpoint
	servicePrincipalToken, err := GetServicePrincipalToken(tenantId, env, rmEndPoint, useManagedIdentityExtension, aADClientSecret, aADClientID)
	if err != nil {
		return nil, err
	}
	authorizer = autorest.NewBearerAuthorizer(servicePrincipalToken)
	return authorizer, nil

}

func GetKeyvaultToken(grantType OAuthGrantType, cloudName string, tenantId string, useManagedIdentityExtension bool, aADClientSecret string, aADClientID string) (authorizer autorest.Authorizer, err error) {
	
	env, err := ParseAzureEnvironment(cloudName)
	if err != nil {
		return nil, err
	}

	kvEndPoint := env.KeyVaultEndpoint
	if '/' == kvEndPoint[len(kvEndPoint)-1] {
		kvEndPoint = kvEndPoint[:len(kvEndPoint)-1]
	}
	servicePrincipalToken, err := GetServicePrincipalToken(tenantId, env, kvEndPoint, useManagedIdentityExtension, aADClientSecret, aADClientID)
	if err != nil {
		return nil, err
	}
	authorizer = autorest.NewBearerAuthorizer(servicePrincipalToken)
	return authorizer, nil
	

}

// GetServicePrincipalToken creates a new service principal token based on the configuration
func GetServicePrincipalToken(tenantId string, env *azure.Environment, resource string, useManagedIdentityExtension bool, aADClientSecret string, aADClientID string) (*adal.ServicePrincipalToken, error) {
	oauthConfig, err := adal.NewOAuthConfig(env.ActiveDirectoryEndpoint, tenantId)
	if err != nil {
		return nil, fmt.Errorf("creating the OAuth config: %v", err)
	}

	if useManagedIdentityExtension {
		glog.V(2).Infoln("azure: using managed identity extension to retrieve access token")
		msiEndpoint, err := adal.GetMSIVMEndpoint()
		if err != nil {
			return nil, fmt.Errorf("Getting the managed service identity endpoint: %v", err)
		}
		return adal.NewServicePrincipalTokenFromMSI(
			msiEndpoint,
			resource)
	}

	if len(aADClientSecret) > 0 {
		glog.V(2).Infoln("azure: using client_id+client_secret to retrieve access token")
		return adal.NewServicePrincipalToken(
			*oauthConfig,
			aADClientID,
			aADClientSecret,
			resource)
	}

	return nil, fmt.Errorf("No credentials provided for AAD application %s", aADClientID)
}

// ParseAzureEnvironment returns azure environment by name
func ParseAzureEnvironment(cloudName string) (*azure.Environment, error) {
	var env azure.Environment
	var err error
	if cloudName == "" {
		env = azure.PublicCloud
	} else {
		env, err = azure.EnvironmentFromName(cloudName)
	}
	return &env, err
}
