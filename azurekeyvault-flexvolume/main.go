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
	program                 = "azurekeyvault-flexvolume"
	version                 = "0.0.3"
	permission  os.FileMode = 0644
)

// Type of Azure Key Vault objects
const (
	// VaultTypeSecret secret vault object type
	VaultTypeSecret          string = "secret"
	// VaultTypeKey key vault object type
	VaultTypeKey             string = "key"
	// VaultTypeCertificate certificate vault object type
	VaultTypeCertificate     string = "cert"
) 

// Option is a collection of configs
type Option struct {
	// the name of the Azure Key Vault instance
	vaultName string
	// the name of the Azure Key Vault object
	vaultObjectName string
	// the version of the Azure Key Vault object
	vaultObjectVersion string
	// the type of the Azure Key Vault object
	vaultObjectType string
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
		fmt.Printf("\n %s\n", err)
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

	vaultUrl, err := getVault(ctx, options.subscriptionId, options.vaultName, options.resourceGroup)
	if err != nil {
		showError("failed to get key vault, error: %s", err)
		fmt.Printf("\n failed to get key vault \n")
		os.Exit(1)
	}

	token, err := GetKeyvaultToken(AuthGrantType(), options.cloudName, options.tenantId, options.usePodIdentity, options.aADClientSecret, options.aADClientID, options.podName, options.podNamespace)
	if err != nil {
		showError("failed to get key vault token, error: %s", err)
		fmt.Printf("\n failed to get key vault token \n")
		os.Exit(1)
	}
	
	kvClient.Authorizer = token
	glog.V(0).Infof("accessing %s %s", options.vaultObjectType, options.vaultObjectName)
	switch options.vaultObjectType {
	case VaultTypeSecret:
		secret, err := kvClient.GetSecret(ctx, *vaultUrl, options.vaultObjectName, options.vaultObjectVersion)
		if err != nil {
			handleError(options.vaultObjectType, options.vaultObjectName, err)
		}
		writeContent([]byte(*secret.Value), options.vaultObjectType, options.vaultObjectName)
	case VaultTypeKey:
		keybundle, err := kvClient.GetKey(ctx, *vaultUrl, options.vaultObjectName, options.vaultObjectVersion)
		if err != nil {
			handleError(options.vaultObjectType, options.vaultObjectName, err)
		}
		// NOTE: we are writing the RSA modulus content of the key 
		writeContent([]byte(*keybundle.Key.N), options.vaultObjectType, options.vaultObjectName)
	case VaultTypeCertificate:
		certbundle, err := kvClient.GetCertificate(ctx, *vaultUrl, options.vaultObjectName, options.vaultObjectVersion)
		if err != nil {
			handleError(options.vaultObjectType, options.vaultObjectName, err)
		}
		writeContent(*certbundle.Cer, options.vaultObjectType, options.vaultObjectName)
	default:
		showError("invalid vaultObjectType")
		fmt.Printf("\n invalid vaultObjectType, should be secret, key, or cert \n")
		os.Exit(1)
	}
	

	os.Exit(0)
}

func handleError (objectType string, objectName string, err error) {
	showError("failed to get %s %s, error: %s",  objectType, err)
	fmt.Printf("\n failed to get %s %s\n", objectType, objectName)
	os.Exit(1)
}

func writeContent(objectContent []byte, objectType string, objectName string) {
	var err error
	if err = ioutil.WriteFile(path.Join(options.dir, objectName), objectContent, permission); err != nil {
		showError("azure KeyVault failed to write %s %s at %s with err %s", objectType, objectName, options.dir, err)
		fmt.Printf("\n azure KeyVault failed to write %s %s at %s \n", objectType, objectName, options.dir)
		os.Exit(1)
	}
	glog.V(0).Infof("azure KeyVault wrote %s %s at %s", objectType, objectName, options.dir)
}

func parseConfigs() error {
	flag.StringVar(&options.vaultName, "vaultName", "", "Name of Azure Key Vault instance.")
	flag.StringVar(&options.vaultObjectName, "vaultObjectName", "", "Name of Azure Key Vault object.")
	
	flag.StringVar(&options.vaultObjectType, "vaultObjectType", "", "Type of Azure Key Vault object.")
	flag.StringVar(&options.vaultObjectVersion, "vaultObjectVersion", "", "Version of Azure Key Vault object.")
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
	if options.vaultObjectVersion == "" {
		return fmt.Errorf("-vaultObjectVersion is not set")
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

	if options.vaultObjectType != VaultTypeSecret && options.vaultObjectType != VaultTypeKey && options.vaultObjectType != VaultTypeCertificate {
		return fmt.Errorf("-vaultObjectType is invalid, should be set to secret, key, or certificate")
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