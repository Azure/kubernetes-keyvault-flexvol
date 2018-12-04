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
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	kv "github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	kvmgmt "github.com/Azure/azure-sdk-for-go/services/keyvault/mgmt/2016-10-01/keyvault"
)

const (
	program                = "azurekeyvault-flexvolume"
	version                = "0.0.6"
	permission os.FileMode = 0644
	objectsSep             = ";"
)

// Type of Azure Key Vault objects
const (
	// VaultTypeSecret secret vault object type
	VaultTypeSecret string = "secret"
	// VaultTypeKey key vault object type
	VaultTypeKey string = "key"
	// VaultTypeCertificate certificate vault object type
	VaultTypeCertificate string = "cert"
)

// Option is a collection of configs
type Option struct {
	// the name of the Azure Key Vault instance
	vaultName string
	// the name of the Azure Key Vault objects
	vaultObjectNames string
	// the versions of the Azure Key Vault objects
	vaultObjectVersions string
	// the types of the Azure Key Vault objects
	vaultObjectTypes string
	// the resourcegroup of the Azure Key Vault
	resourceGroup string
	// directory to save the vault objects
	dir string
	// subscriptionId to azure
	subscriptionId string
	// version flag
	showVersion bool
	// cloud name
	cloudName string
	// tenantID in AAD
	tenantId string
	// POD AAD Identity flag
	usePodIdentity bool
	// AAD app client secret (if not using POD AAD Identity)
	aADClientSecret string
	// AAD app client secret id (if not using POD AAD Identity)
	aADClientID string
	// the name of the pod (if using POD AAD Identity)
	podName string
	// the namespace of the pod (if using POD AAD Identity)
	podNamespace string
}

// KeyvaultFlexvolumeAdapter encapsulates the logic to connect to keyvault using provided identity,
// extract keys, secrets and certificate and write them on disk in the provided directory.
type KeyvaultFlexvolumeAdapter struct {
	ctx     context.Context
	options Option
}

func main() {
	context := context.Background()
	options, err := parseConfigs()
	if err != nil {
		glog.Fatalf("[error] : %s", err)
	}

	adapter := &KeyvaultFlexvolumeAdapter{ctx: context, options: *options}
	err = adapter.Run()
	if err != nil {
		glog.Fatalf("[error] : %s", err)
	}
	os.Exit(0)
}

//Run fetches the specified objects from keyvault and writes them on dir
func (adapter *KeyvaultFlexvolumeAdapter) Run() error {
	options := adapter.options
	ctx := adapter.ctx
	if options.showVersion {
		fmt.Printf("%s %s\n", program, version)
		fmt.Printf("%s \n", options.subscriptionId)
	}

	_, err := os.Lstat(options.dir)
	if err != nil {
		return errors.Wrapf(err, "failed to get directory %s", options.dir)
	}

	glog.Infof("starting the %s, %s", program, version)

	vaultUrl, err := adapter.getVaultURL()
	if err != nil {
		return errors.Wrap(err, "failed to get vault")
	}

	kvClient, err := adapter.initializeKvClient()
	if err != nil {
		return errors.Wrap(err, "failed to get keyvaultClient")
	}

	objectTypes := strings.Split(options.vaultObjectTypes, objectsSep)
	objectNames := strings.Split(options.vaultObjectNames, objectsSep)
	numOfObjects := len(objectNames)

	// objectVersions are optional so we take as much as we can
	objectVersions := make([]string, numOfObjects)
	for index, value := range strings.Split(options.vaultObjectVersions, objectsSep) {
		objectVersions[index] = value
	}

	for i := 0; i < numOfObjects; i++ {
		objectType := objectTypes[i]
		objectName := objectNames[i]
		objectVersion := objectVersions[i]

		glog.V(0).Infof("retrieving %s %s (version: %s)", objectType, objectName, objectVersion)
		switch objectType {
		case VaultTypeSecret:
			secret, err := kvClient.GetSecret(ctx, *vaultUrl, objectName, objectVersion)
			if err != nil {
				return wrapObjectTypeError(err, objectType, objectName, objectVersion)
			}
			return writeContent([]byte(*secret.Value), objectType, objectName, options.dir)
		case VaultTypeKey:
			keybundle, err := kvClient.GetKey(ctx, *vaultUrl, objectName, objectVersion)
			if err != nil {
				return wrapObjectTypeError(err, objectType, objectName, objectVersion)
			}
			// NOTE: we are writing the RSA modulus content of the key
			return writeContent([]byte(*keybundle.Key.N), objectType, objectName, options.dir)
		case VaultTypeCertificate:
			certbundle, err := kvClient.GetCertificate(ctx, *vaultUrl, objectName, objectVersion)
			if err != nil {
				return wrapObjectTypeError(err, objectType, objectName, objectVersion)
			}
			return writeContent(*certbundle.Cer, objectType, objectName, options.dir)
		default:
			return errors.Errorf("Invalid vaultObjectTypes. Should be secret, key, or cert")
		}
	}

	return nil
}

func (adapter *KeyvaultFlexvolumeAdapter) initializeKvClient() (*kv.BaseClient, error) {
	kvClient := kv.New()
	options := adapter.options

	token, err := GetKeyvaultToken(AuthGrantType(), options.cloudName, options.tenantId, options.usePodIdentity, options.aADClientSecret, options.aADClientID, options.podName, options.podNamespace)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get key vault token")
	}

	kvClient.Authorizer = token
	return &kvClient, nil
}

func wrapObjectTypeError(err error, objectType string, objectName string, objectVersion string) error {
	return errors.Wrapf(err, "failed to get objectType:%s, objetName:%s, objectVersion:%s", objectType, objectName, objectVersion)
}

func writeContent(objectContent []byte, objectType string, objectName string, dir string) error {
	if err := ioutil.WriteFile(path.Join(dir, objectName), objectContent, permission); err != nil {
		return errors.Wrapf(err, "azure KeyVault failed to write %s %s at %s", objectType, objectName, dir)
	}
	glog.V(0).Infof("azure KeyVault wrote %s %s at %s", objectType, objectName, dir)
	return nil
}

func parseConfigs() (*Option, error) {
	var options Option
	flag.StringVar(&options.vaultName, "vaultName", "", "Name of Azure Key Vault instance.")
	flag.StringVar(&options.vaultObjectNames, "vaultObjectNames", "", "Names of Azure Key Vault objects, semi-colon separated.")
	flag.StringVar(&options.vaultObjectTypes, "vaultObjectTypes", "", "Types of Azure Key Vault objects, semi-colon separated.")
	flag.StringVar(&options.vaultObjectVersions, "vaultObjectVersions", "", "Versions of Azure Key Vault objects, semi-colon separated.")
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
		return nil, fmt.Errorf("-vaultName is not set")
	}

	if options.vaultObjectNames == "" {
		return nil, fmt.Errorf("-vaultObjectNames is not set")
	}

	if options.resourceGroup == "" {
		return nil, fmt.Errorf("-resourceGroup is not set")
	}

	if options.subscriptionId == "" {
		return nil, fmt.Errorf("-subscriptionId is not set")
	}

	if options.dir == "" {
		return nil, fmt.Errorf("-dir is not set")
	}

	if options.tenantId == "" {
		return nil, fmt.Errorf("-tenantId is not set")
	}

	if strings.Count(options.vaultObjectNames, objectsSep) !=
		strings.Count(options.vaultObjectTypes, objectsSep) {
		return nil, fmt.Errorf("-vaultObjectNames and -vaultObjectTypes are not matching")
	}

	if options.usePodIdentity == false {
		if options.aADClientID == "" {
			return nil, fmt.Errorf("-aADClientID is not set")
		}
		if options.aADClientSecret == "" {
			return nil, fmt.Errorf("-aADClientSecret is not set")
		}
	} else {
		if options.podName == "" {
			return nil, fmt.Errorf("-podName is not set")
		}
		if options.podNamespace == "" {
			return nil, fmt.Errorf("-podNamespace is not set")
		}
	}

	// validate all object types
	for _, objectType := range strings.Split(options.vaultObjectTypes, objectsSep) {
		if objectType != VaultTypeSecret && objectType != VaultTypeKey && objectType != VaultTypeCertificate {
			return nil, fmt.Errorf("-vaultObjectType is invalid, should be set to secret, key, or certificate")
		}
	}
	return &options, nil
}

func (adapter *KeyvaultFlexvolumeAdapter) getVaultURL() (vaultUrl *string, err error) {
	glog.Infof("subscriptionID: %s", adapter.options.subscriptionId)
	glog.Infof("vaultName: %s", adapter.options.vaultName)
	glog.Infof("resourceGroup: %s", adapter.options.resourceGroup)

	vaultsClient := kvmgmt.NewVaultsClient(adapter.options.subscriptionId)
	token, tokenErr := GetManagementToken(AuthGrantType(),
		adapter.options.cloudName,
		adapter.options.tenantId,
		adapter.options.usePodIdentity,
		adapter.options.aADClientSecret,
		adapter.options.aADClientID,
		adapter.options.podName,
		adapter.options.podNamespace)
	if tokenErr != nil {
		return nil, errors.Wrapf(err, "failed to get management token")
	}
	vaultsClient.Authorizer = token
	vault, err := vaultsClient.Get(adapter.ctx, adapter.options.resourceGroup, adapter.options.vaultName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get vault %s", adapter.options.vaultName)
	}
	return vault.Properties.VaultURI, nil
}
