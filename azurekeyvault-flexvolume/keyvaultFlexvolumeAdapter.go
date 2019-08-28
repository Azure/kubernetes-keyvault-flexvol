// Copyright (c) Microsoft and contributors.  All rights reserved.
//
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	kv "github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	kvmgmt "github.com/Azure/azure-sdk-for-go/services/keyvault/mgmt/2016-10-01/keyvault"
	"github.com/golang/glog"
	"github.com/pkg/errors"
)

// KeyvaultFlexvolumeAdapter encapsulates the logic to connect to keyvault using provided identity,
// extract keys, secrets and certificate and write them on disk in the provided directory.
type KeyvaultFlexvolumeAdapter struct {
	ctx     context.Context
	options Option
}

//Run fetches the specified objects from keyvault and writes them on dir
func (adapter *KeyvaultFlexvolumeAdapter) Run() error {
	options := adapter.options
	ctx := adapter.ctx
	if options.showVersion {
		glog.V(0).Infof("%s %s", program, version)
		glog.V(2).Infof("%s", options.subscriptionID)
	}

	_, err := os.Lstat(options.dir)
	if err != nil {
		return errors.Wrapf(err, "failed to get directory %s", options.dir)
	}

	glog.Infof("starting the %s, %s", program, version)

	vaultURL, err := adapter.getVaultURL()
	if err != nil {
		return errors.Wrap(err, "failed to get vault")
	}
	if vaultURL == nil {
		return fmt.Errorf("vault url is nil")
	}

	kvClient, err := adapter.initializeKvClient()
	if err != nil {
		return errors.Wrap(err, "failed to get keyvaultClient")
	}

	objectTypes := strings.Split(options.vaultObjectTypes, objectsSep)
	objectNames := strings.Split(options.vaultObjectNames, objectsSep)
	objectAliases := strings.Split(options.vaultObjectAliases, objectsSep)
	objectVersions := strings.Split(options.vaultObjectVersions, objectsSep)

	for i := range objectNames {
		objectType := objectTypes[i]
		objectName := objectNames[i]
		// default to the objectName and override if aliases are available
		fileName := path.Join(options.dir, objectNames[i])
		if options.vaultObjectAliases != "" && len(objectAliases) == len(objectNames) {
			fileName = path.Join(options.dir, objectAliases[i])
		}
		// objectVersions are optional so we take as much as we can
		objectVersion := ""
		if options.vaultObjectVersions != "" && len(objectVersions) == len(objectNames) {
			objectVersion = objectVersions[i]
		}
		glog.V(0).Infof("retrieving %s %s (version: %s)", objectType, objectName, objectVersion)
		switch objectType {
		case VaultTypeSecret:
			secret, err := kvClient.GetSecret(ctx, *vaultURL, objectName, objectVersion)
			if err != nil {
				return wrapObjectTypeError(err, objectType, objectName, objectVersion)
			}
			if err = ioutil.WriteFile(fileName, []byte(*secret.Value), permission); err != nil {
				return errors.Wrapf(err, "azure KeyVault failed to write secret %s to %s", objectName, fileName)
			}
		case VaultTypeKey:
			keybundle, err := kvClient.GetKey(ctx, *vaultURL, objectName, objectVersion)
			if err != nil {
				return wrapObjectTypeError(err, objectType, objectName, objectVersion)
			}
			// NOTE: we are writing the RSA modulus content of the key
			if err = ioutil.WriteFile(fileName, []byte(*keybundle.Key.N), permission); err != nil {
				return errors.Wrapf(err, "azure KeyVault failed to write key %s to %s", objectName, fileName)
			}
		case VaultTypeCertificate:
			certbundle, err := kvClient.GetCertificate(ctx, *vaultURL, objectName, objectVersion)
			if err != nil {
				return wrapObjectTypeError(err, objectType, objectName, objectVersion)
			}
			if err = ioutil.WriteFile(fileName, *certbundle.Cer, permission); err != nil {
				return errors.Wrapf(err, "azure KeyVault failed to write certificate %s to %s", objectName, fileName)
			}
		default:
			err = errors.Errorf("Invalid vaultObjectTypes. Should be secret, key, or cert")
			return wrapObjectTypeError(err, objectType, objectName, objectVersion)
		}
		glog.V(0).Infof("azure KeyVault wrote %s %s at %s", objectType, objectName, fileName)
	}
	return nil
}

func (adapter *KeyvaultFlexvolumeAdapter) initializeKvClient() (*kv.BaseClient, error) {
	kvClient := kv.New()
	options := adapter.options

	token, err := GetKeyvaultToken(AuthGrantType(), options.cloudName, options.tenantID, options.usePodIdentity, options.aADClientSecret, options.aADClientID, options.podName, options.podNamespace)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get key vault token")
	}

	kvClient.Authorizer = token
	return &kvClient, nil
}

func wrapObjectTypeError(err error, objectType string, objectName string, objectVersion string) error {
	return errors.Wrapf(err, "failed to get objectType:%s, objectName:%s, objectVersion:%s", objectType, objectName, objectVersion)
}

func (adapter *KeyvaultFlexvolumeAdapter) getVaultURL() (vaultURL *string, err error) {
	glog.V(2).Infof("subscriptionID: %s", adapter.options.subscriptionID)
	glog.V(2).Infof("vaultName: %s", adapter.options.vaultName)
	glog.V(2).Infof("resourceGroup: %s", adapter.options.resourceGroup)

	vaultsClient := kvmgmt.NewVaultsClient(adapter.options.subscriptionID)
	token, tokenErr := GetManagementToken(AuthGrantType(),
		adapter.options.cloudName,
		adapter.options.tenantID,
		adapter.options.usePodIdentity,
		adapter.options.aADClientSecret,
		adapter.options.aADClientID,
		adapter.options.podName,
		adapter.options.podNamespace)
	if tokenErr != nil {
		return nil, errors.Wrap(tokenErr, "failed to get management token")
	}
	vaultsClient.Authorizer = token
	vault, err := vaultsClient.Get(adapter.ctx, adapter.options.resourceGroup, adapter.options.vaultName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get vault %s", adapter.options.vaultName)
	}
	return vault.Properties.VaultURI, nil
}
