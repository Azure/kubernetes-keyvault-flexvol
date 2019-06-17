// Copyright (c) Microsoft and contributors.  All rights reserved.
//
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/pkg/errors"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/golang/glog"
)

const (
	nmiendpoint   = "http://localhost:2579/host/token/"
	podnameheader = "podname"
	podnsheader   = "podns"
)

var (
	oauthConfig *adal.OAuthConfig
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
	// Use managed service identity integrated with pod identity to get access to Azure ARM resources
	UsePodIdentity bool `json:"usePodIdentity"`
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

type NMIResponse struct {
	Token    adal.Token `json:"token"`
	ClientID string     `json:"clientid"`
}

func GetManagementToken(grantType OAuthGrantType, cloudName string, tenantId string, usePodIdentity bool, aADClientSecret string, aADClientID string, podname string, podns string) (authorizer autorest.Authorizer, err error) {

	env, err := ParseAzureEnvironment(cloudName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse Azure environment")
	}

	rmEndPoint := env.ResourceManagerEndpoint
	servicePrincipalToken, err := GetServicePrincipalToken(tenantId, env, rmEndPoint, usePodIdentity, aADClientSecret, aADClientID, podname, podns)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get service principal token")
	}
	authorizer = autorest.NewBearerAuthorizer(servicePrincipalToken)
	return authorizer, nil

}

func GetKeyvaultToken(grantType OAuthGrantType, cloudName string, tenantId string, usePodIdentity bool, aADClientSecret string, aADClientID string, podname string, podns string) (authorizer autorest.Authorizer, err error) {

	env, err := ParseAzureEnvironment(cloudName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse Azure environment")
	}

	kvEndPoint := env.KeyVaultEndpoint
	if '/' == kvEndPoint[len(kvEndPoint)-1] {
		kvEndPoint = kvEndPoint[:len(kvEndPoint)-1]
	}
	servicePrincipalToken, err := GetServicePrincipalToken(tenantId, env, kvEndPoint, usePodIdentity, aADClientSecret, aADClientID, podname, podns)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get service principal token")
	}
	authorizer = autorest.NewBearerAuthorizer(servicePrincipalToken)
	return authorizer, nil

}

// GetServicePrincipalToken creates a new service principal token based on the configuration
func GetServicePrincipalToken(tenantId string, env *azure.Environment, resource string, usePodIdentity bool, aADClientSecret string, aADClientID string, podname string, podns string) (*adal.ServicePrincipalToken, error) {
	oauthConfig, err := adal.NewOAuthConfig(env.ActiveDirectoryEndpoint, tenantId)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating the OAuth config")
	}

	// For usepodidentity mode, the flexvolume driver makes an authorization request to fetch token for a resource from the NMI host endpoint (http://127.0.0.1:2579/host/token/).
	// The request includes the pod namespace `podns` and the pod name `podname` in the request header and the resource endpoint of the resource requesting the token.
	// The NMI server identifies the pod based on the `podns` and `podname` in the request header and then queries k8s (through MIC) for a matching azure identity.
	// Then nmi makes an adal request to get a token for the resource in the request, returns the `token` and the `clientid` as a reponse to the flexvolume request.

	if usePodIdentity {
		glog.V(0).Infoln("azure: using pod identity to retrieve token")

		endpoint := fmt.Sprintf("%s?resource=%s", nmiendpoint, resource)
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Add(podnsheader, podns)
		req.Header.Add(podnameheader, podname)

		resp, err := retryFetchToken(req, 5)
		if err != nil {
			return nil, errors.Wrap(err, "failed to query NMI")
		}
		if resp == nil {
			return nil, fmt.Errorf("nmi response is nil")
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fmt.Print("failed to close NMI response body")
			}
		}()

		if resp.StatusCode == http.StatusOK {
			var nmiResp = NMIResponse{}
			if err := json.NewDecoder(resp.Body).Decode(&nmiResp); err != nil {
				return nil, errors.Wrap(err, "failed to decode NMI response")
			}

			r, _ := regexp.Compile("^(\\S{4})(\\S|\\s)*(\\S{4})$")
			fmt.Printf("\n accesstoken: %s\n", r.ReplaceAllString(nmiResp.Token.AccessToken, "$1##### REDACTED #####$3"))
			fmt.Printf("\n clientid: %s\n", r.ReplaceAllString(nmiResp.ClientID, "$1##### REDACTED #####$3"))

			token := nmiResp.Token
			clientID := nmiResp.ClientID

			if &token == nil || clientID == "" {
				return nil, fmt.Errorf("nmi did not return expected values in response: token and clientid")
			}

			spt, err := adal.NewServicePrincipalTokenFromManualToken(*oauthConfig, clientID, resource, token, nil)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get new service principal token from manual token")
			}
			return spt, nil
		}

		return nil, fmt.Errorf("nmi response failed with status code: %d", resp.StatusCode)
	}
	// When flexvolume driver is using a Service Principal clientid + client secret to retrieve token for resource
	if len(aADClientSecret) > 0 {
		glog.V(2).Infoln("azure: using client_id+client_secret to retrieve access token")
		return adal.NewServicePrincipalToken(
			*oauthConfig,
			aADClientID,
			aADClientSecret,
			resource)
	}

	return nil, fmt.Errorf("no credentials provided for AAD application %s", aADClientID)
}

func retryFetchToken(req *http.Request, maxAttempts int) (resp *http.Response, err error) {
	attempt := 0
	// Not using exponential backoff logic because the avg time taken by nmi to complete
	// identity assignment is ~35s. With exponential backoff we might end up waiting
	// longer than required. Kubelet poll interval is 300ms which is aggressive and results in
	// a lot of failed volume mount events. This retry will reduce the number of events in the
	// case when nmi takes longer than usual.
	delay := time.Duration(7 * time.Second)
	client := &http.Client{}
	for attempt < maxAttempts {
		resp, err = client.Do(req)

		// pod-identity calls will be retried in every scenario except when the err is nil
		// and we get 200 response code.
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			return
		}

		attempt++

		select {
		case <-time.After(delay):
		case <-req.Context().Done():
			// request is cancelled
			err = req.Context().Err()
			return nil, err
		}
	}
	return
}

// ParseAzureEnvironment returns azure environment by name
func ParseAzureEnvironment(cloudName string) (*azure.Environment, error) {
	if cloudName == "" {
		return &azure.PublicCloud, nil
	}
	env, err := azure.EnvironmentFromName(cloudName)
	return &env, errors.Wrapf(err, "failed to get environment from cloudName: %s", cloudName)
}
