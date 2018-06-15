// Copyright (c) Microsoft and contributors.  All rights reserved.
//
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"github.com/golang/glog"
	
	kvmgmt "github.com/Azure/azure-sdk-for-go/services/keyvault/mgmt/2016-10-01/keyvault"
	kv "github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	
)

const (
	program					= "azurekeyvault-flexvolume"
	version     			= "0.0.1"
	permission  os.FileMode = 0644
	cache					= 60
)

// Option is a collection of configs
type Option struct {
	// the name of the Azure Key Vault instance
	vaultName string
	// the name of the Azure Key Vault secret
	secretName string
	// the resourcegroup of the Azure Key Vault
	resourceGroup string
	// directory to save data
	dir string
	// subscriptionId to azure
	subscriptionId string
	// version flag
	showVersion bool
	cloudName string
	tenantId string 
	useManagedIdentityExtension bool
	aADClientSecret string
	aADClientID string
}

var (
	options Option
)

func main() {
	ctx := context.Background()

	if err := parseConfigs(); err != nil {
		showUsage("invalid config, %s", err)
	}
	
	if options.showVersion {
		fmt.Printf("%s %s\n", program, version)
		fmt.Printf("%s \n", options.subscriptionId)
	}
	_, err := os.Lstat(options.dir)
	if err != nil {
		showError("failed to get directory %s, error: %s", options.dir, err)
	}

	glog.Infof("starting the %s, %s", program, version)
	kvClient := kv.New()

	content := []byte("")

	fileInfo, err := os.Lstat(path.Join(options.dir, options.secretName + ".txt"))
	if fileInfo != nil && err == nil {
		glog.V(0).Infof("secret %s already exists in %s", options.secretName,options.dir)
		content, err = ioutil.ReadFile(path.Join(options.dir, options.secretName))
		if err != nil {
			showError("failed to read content from file, error: %s", err)
		}
	} 
	vaultUrl, err := getVault(ctx, options.subscriptionId, options.vaultName, options.resourceGroup)
	if err != nil {
		showError("failed to get key vault, error: %s", err)
	}

	token, err := GetKeyvaultToken(AuthGrantType(), options.cloudName, options.tenantId, options.useManagedIdentityExtension, options.aADClientSecret, options.aADClientID)
	if err != nil {
		showError("failed to get token, error: %s", err)
	}
	
	kvClient.Authorizer = token

	secret, err := kvClient.GetSecret(ctx, *vaultUrl, options.secretName, "")
	if err != nil {
		showError("failed to get secret, error: %s", err)
	}
	if string(content) == *secret.Value {
		glog.V(0).Infof("secret %s content has not been updated", options.secretName)
	} else {
		if err = ioutil.WriteFile(path.Join(options.dir, options.secretName + ".txt"), []byte(*secret.Value), permission); err != nil {
			showError("azure KeyVault failed to write secret %s at %s with err %s", options.secretName, options.dir, err)
		}
		glog.V(0).Infof("azure KeyVault wrote secret %s at %s", options.secretName,options.dir)
	}	
}

func parseConfigs() error {
	flag.StringVar(&options.vaultName, "vaultName", "", "Name of Azure Key Vault instance.")
	flag.StringVar(&options.secretName, "secretName", "", "Name of Azure Key Vault secret.")
	flag.StringVar(&options.resourceGroup, "resourceGroup", "", "Resource group name of Azure Key Vault.")
	flag.StringVar(&options.subscriptionId, "subscriptionId", "", "subscriptionId to Azure.")
	flag.StringVar(&options.aADClientID, "aADClientID", "", "aADClientID to Azure.")
	flag.StringVar(&options.aADClientSecret, "aADClientSecret", "", "aADClientSecret to Azure.")
	flag.StringVar(&options.cloudName, "cloudName", "", "Type of Azure cloud")
	flag.StringVar(&options.tenantId, "tenantId", "", "tenantId to Azure")
	flag.BoolVar(&options.useManagedIdentityExtension, "useManagedIdentityExtension", false, "useManagedIdentityExtension for MSI")
	flag.StringVar(&options.dir, "dir", "", "Directory path to write data.")
	flag.BoolVar(&options.showVersion, "version", true, "Show version.")

	flag.Parse()
	fmt.Println(options.vaultName)

	if options.vaultName == "" {
		return fmt.Errorf("VAULT_NAME is unset")
	}
	if options.secretName == "" {
		return fmt.Errorf("SECRET_NAME is unset")
	}
	if options.resourceGroup == "" {
		return fmt.Errorf("RESOURCE_GROUP is unset")
	}
	if options.subscriptionId == "" {
		return fmt.Errorf("SUBSCRIPTION_ID is unset")
	}
	if options.dir == "" {
		return fmt.Errorf("DIR is unset")
	}
	return nil
}

func getEnv(variable, value string) string {
	if v := os.Getenv(variable); v != "" {
		return v
	}

	return value
}

func showUsage(message string, args ...interface{}) {
	flag.PrintDefaults()
	if message != "" {
		fmt.Printf("\n[error] "+message+"\n", args...)
		os.Exit(1)
	}

	os.Exit(0)
}

func showError(message string, args ...interface{}) {
	if message != "" {
		fmt.Printf("\n[error] "+message+"\n", args...)
		os.Exit(1)
	}

	os.Exit(0)
}

func getVault(ctx context.Context, subscriptionID string, vaultName string, resourceGroup string) (vaultUrl *string, err error) {
	glog.Infof("subscriptionID: %s", subscriptionID)
	glog.Infof("vaultName: %s", vaultName)
	glog.Infof("resourceGroup: %s", resourceGroup)

	vaultsClient := kvmgmt.NewVaultsClient(subscriptionID)
	token, _ := GetManagementToken(AuthGrantType(), options.cloudName, options.tenantId, options.useManagedIdentityExtension, options.aADClientSecret, options.aADClientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get management token, error: %v", err)
	}
	vaultsClient.Authorizer = token
	vault, err := vaultsClient.Get(ctx, resourceGroup, vaultName)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault, error: %v", err)
	}
	return vault.Properties.VaultURI, nil
}