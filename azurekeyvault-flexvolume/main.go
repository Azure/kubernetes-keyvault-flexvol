// Copyright (c) Microsoft and contributors.  All rights reserved.
//
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/golang/glog"
)

const (
	program                = "azurekeyvault-flexvolume"
	version                = "0.0.12"
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
	// the filenames the objects will be written to
	vaultObjectAliases string
	// the versions of the Azure Key Vault objects
	vaultObjectVersions string
	// the types of the Azure Key Vault objects
	vaultObjectTypes string
	// the resourcegroup of the Azure Key Vault
	resourceGroup string
	// directory to save the vault objects
	dir string
	// subscriptionID to azure
	subscriptionID string
	// version flag
	showVersion bool
	// cloud name
	cloudName string
	// tenantID in AAD
	tenantID string
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

func main() {
	ctx := context.Background()
	options, err := parseConfigs()
	if err != nil {
		glog.Errorf("[error] : %s", err)
		os.Exit(1)
	}

	adapter := &KeyvaultFlexvolumeAdapter{ctx: ctx, options: *options}
	err = adapter.Run()
	if err != nil {
		glog.Fatalf("[error] : %s", err)
	}
	glog.Flush()
	os.Exit(0)
}

func parseConfigs() (*Option, error) {
	var options Option
	flag.StringVar(&options.vaultName, "vaultName", "", "Name of Azure Key Vault instance.")
	flag.StringVar(&options.vaultObjectNames, "vaultObjectNames", "", "Names of Azure Key Vault objects, semi-colon separated.")
	flag.StringVar(&options.vaultObjectAliases, "vaultObjectAliases", "", "Filenames to write the Azure Key Vault objects to, semi-colon separated.")
	flag.StringVar(&options.vaultObjectTypes, "vaultObjectTypes", "", "Types of Azure Key Vault objects, semi-colon separated.")
	flag.StringVar(&options.vaultObjectVersions, "vaultObjectVersions", "", "Versions of Azure Key Vault objects, semi-colon separated.")
	flag.StringVar(&options.resourceGroup, "resourceGroup", "", "Resource group name of Azure Key Vault.")
	flag.StringVar(&options.subscriptionID, "subscriptionId", "", "subscriptionId to Azure.")
	flag.StringVar(&options.aADClientID, "aADClientID", "", "aADClientID to Azure.")
	flag.StringVar(&options.aADClientSecret, "aADClientSecret", "", "aADClientSecret to Azure.")
	flag.StringVar(&options.cloudName, "cloudName", "", "Type of Azure cloud")
	flag.StringVar(&options.tenantID, "tenantId", "", "tenantId to Azure")
	flag.BoolVar(&options.usePodIdentity, "usePodIdentity", false, "usePodIdentity for using pod identity.")
	flag.StringVar(&options.dir, "dir", "", "Directory path to write data.")
	flag.BoolVar(&options.showVersion, "version", true, "Show version.")
	flag.StringVar(&options.podName, "podName", "", "Name of the pod")
	flag.StringVar(&options.podNamespace, "podNamespace", "", "Namespace of the pod")

	flag.Parse()

	err := Validate(options)
	return &options, err
}

// Validate volume options
func Validate(options Option) error {
	if options.vaultName == "" {
		return fmt.Errorf("-vaultName is not set")
	}

	if options.vaultObjectNames == "" {
		return fmt.Errorf("-vaultObjectNames is not set")
	}

	if options.resourceGroup == "" {
		return fmt.Errorf("-resourceGroup is not set")
	}

	if options.subscriptionID == "" {
		return fmt.Errorf("-subscriptionId is not set")
	}

	if options.dir == "" {
		return fmt.Errorf("-dir is not set")
	}

	if options.tenantID == "" {
		return fmt.Errorf("-tenantId is not set")
	}

	if strings.Count(options.vaultObjectNames, objectsSep) !=
		strings.Count(options.vaultObjectTypes, objectsSep) {
		return fmt.Errorf("-vaultObjectNames and -vaultObjectTypes do not have the same number of items")
	}

	if len(options.vaultObjectAliases) > 0 &&
		(strings.Count(options.vaultObjectAliases, objectsSep) != strings.Count(options.vaultObjectAliases, objectsSep)) {
		return fmt.Errorf("-vaultObjectNames and -vaultObjectAliases do not have the same number of items")
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

	// validate all object types
	for _, objectType := range strings.Split(options.vaultObjectTypes, objectsSep) {
		if objectType != VaultTypeSecret && objectType != VaultTypeKey && objectType != VaultTypeCertificate {
			return fmt.Errorf("-vaultObjectType is invalid, should be set to secret, key, or certificate")
		}
	}

	return nil
}
