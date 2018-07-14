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
	version					= "0.0.2"
	permission  os.FileMode = 0644
	cache					= 60
)

// Option is a collection of configs
type Option struct {
	// the name of the Azure Key Vault instance
	vaultName string
	// the name of the Azure Key Vault object
	vaultObjectName string
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
	usePodIdentity bool
	aADClientSecret string
	aADClientID string
	podName string
	podNamespace string
}

var (
	options Option
)

func main() {
	ctx := context.Background()

	if err := parseConfigs(); err != nil {
		showUsage("invalid config, %s", err)
		fmt.Printf("\n invalid config \n")
		os.Exit(1)
	}
	
	if options.showVersion {
		fmt.Printf("%s %s\n", program, version)
		fmt.Printf("%s \n", options.subscriptionId)
	}
	_, err := os.Lstat(options.dir)
	if err != nil {
		showError("failed to get directory %s, error: %s", options.dir, err)
		fmt.Printf("\n failed to get directory %s \n", options.dir)
		os.Exit(1)
	}

	glog.Infof("starting the %s, %s", program, version)
	kvClient := kv.New()

	content := []byte("")

	fileInfo, err := os.Lstat(path.Join(options.dir, options.vaultObjectName + ".txt"))
	if fileInfo != nil && err == nil {
		glog.V(0).Infof("secret %s already exists in %s", options.vaultObjectName, options.dir)
		content, err = ioutil.ReadFile(path.Join(options.dir, options.vaultObjectName + ".txt"))
		if err != nil {
			showError("failed to read content from file, error: %s", err)
			fmt.Printf("\n failed to read content from file \n")
			os.Exit(1)
		}
	} 
	vaultUrl, err := getVault(ctx, options.subscriptionId, options.vaultName, options.resourceGroup)
	if err != nil {
		showError("failed to get key vault, error: %s", err)
		fmt.Printf("\n failed to get key vault \n")
		os.Exit(1)
	}

	token, err := GetKeyvaultToken(AuthGrantType(), options.cloudName, options.tenantId, options.usePodIdentity, options.aADClientSecret, options.aADClientID, options.podName, options.podNamespace)
	if err != nil {
		showError("failed to get keyvault token, error: %s", err)
		fmt.Printf("\n failed to get keyvault token \n")
		os.Exit(1)
	}
	
	kvClient.Authorizer = token

	secret, err := kvClient.GetSecret(ctx, *vaultUrl, options.vaultObjectName, "")
	if err != nil {
		showError("failed to get secret, error: %s", err)
		fmt.Printf("\n failed to get secret \n")
		os.Exit(1)
	}
	if string(content) == *secret.Value {
		glog.V(0).Infof("secret %s content has not been updated", options.vaultObjectName)
	} else {
		if err = ioutil.WriteFile(path.Join(options.dir, options.vaultObjectName + ".txt"), []byte(*secret.Value), permission); err != nil {
			showError("azure KeyVault failed to write secret %s at %s with err %s", options.vaultObjectName, options.dir, err)
			fmt.Printf("\n azure KeyVault failed to write secret %s at %s \n", options.vaultObjectName, options.dir)
			os.Exit(1)
		}
		glog.V(0).Infof("azure KeyVault wrote secret %s at %s", options.vaultObjectName, options.dir)
	}	

	os.Exit(0)
}

func parseConfigs() error {
	flag.StringVar(&options.vaultName, "vaultName", "", "Name of Azure Key Vault instance.")
	flag.StringVar(&options.vaultObjectName, "vaultObjectName", "", "Name of Azure Key Vault object.")
	flag.StringVar(&options.resourceGroup, "resourceGroup", "", "Resource group name of Azure Key Vault.")
	flag.StringVar(&options.subscriptionId, "subscriptionId", "", "subscriptionId to Azure.")
	flag.StringVar(&options.aADClientID, "aADClientID", "", "aADClientID to Azure.")
	flag.StringVar(&options.aADClientSecret, "aADClientSecret", "", "aADClientSecret to Azure.")
	flag.StringVar(&options.cloudName, "cloudName", "", "Type of Azure cloud")
	flag.StringVar(&options.tenantId, "tenantId", "", "tenantId to Azure")
	flag.BoolVar(&options.usePodIdentity, "usePodIdentity", false, "usePodIdentity for using pod identity.")
	flag.StringVar(&options.dir, "dir", "", "Directory path to write data.")
	flag.BoolVar(&options.showVersion, "version", true, "Show version.")
	flag.StringVar(&options.podName, "podName", "", "Name of the pod")
	flag.StringVar(&options.podNamespace, "podNamespace", "", "Namespace of the pod")

	flag.Parse()
	fmt.Println(options.vaultName)

	if options.vaultName == "" {
		return fmt.Errorf("-vaultName is not set")
	}
	if options.vaultObjectName == "" {
		return fmt.Errorf("-vaultObjectName is not set")
	}
	if options.resourceGroup == "" {
		return fmt.Errorf("-resourceGroup is not set")
	}
	if options.subscriptionId == "" {
		return fmt.Errorf("-subscriptionId is not set")
	}
	if options.dir == "" {
		return fmt.Errorf("-dir is not set")
	}

	if options.usePodIdentity == false {
		if options.aADClientID == "" {
			return fmt.Errorf("-aADClientID is not set")
		}
		if options.aADClientSecret == "" {
			return fmt.Errorf("-aADClientSecret is not set")
		}
	} else {
		if options.podName == "" {
			return fmt.Errorf("-podName is not set")
		}
		if options.podNamespace == "" {
			return fmt.Errorf("-podNamespace is not set")
		}
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
	}
}

func showError(message string, args ...interface{}) {
	if message != "" {
		fmt.Printf("\n[error] "+message+"\n", args...)
	}
}

func getVault(ctx context.Context, subscriptionID string, vaultName string, resourceGroup string) (vaultUrl *string, err error) {
	glog.Infof("subscriptionID: %s", subscriptionID)
	glog.Infof("vaultName: %s", vaultName)
	glog.Infof("resourceGroup: %s", resourceGroup)

	vaultsClient := kvmgmt.NewVaultsClient(subscriptionID)
	token, _ := GetManagementToken(AuthGrantType(), options.cloudName, options.tenantId, options.usePodIdentity, options.aADClientSecret, options.aADClientID, options.podName, options.podNamespace)
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