// Copyright (c) Microsoft and contributors.  All rights reserved.
//
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.
package main

import (
	"context"
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
		glog.V(2).Infof("%s", options.subscriptionId)
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
	objectVersions := strings.Split(options.vaultObjectVersions, objectsSep)

	for i := range objectNames {
		objectType := objectTypes[i]
		objectName := objectNames[i]
		// objectVersions are optional so we take as much as we can
		objectVersion := ""
		if len(objectVersions) == len(objectNames) {
			objectVersion = objectVersions[i]
		}
		glog.V(0).Infof("retrieving %s %s (version: %s)", objectType, objectName, objectVersion)
		switch objectType {
		case VaultTypeSecret:
			secret, err := kvClient.GetSecret(ctx, *vaultUrl, objectName, objectVersion)
			if err != nil {
				return wrapObjectTypeError(err, objectType, objectName, objectVersion)
			}
			if err = writeContent([]byte(*secret.Value), objectType, objectName, options.dir); err != nil {
				return err
			}
		case VaultTypeKey:
			keybundle, err := kvClient.GetKey(ctx, *vaultUrl, objectName, objectVersion)
			if err != nil {
				return wrapObjectTypeError(err, objectType, objectName, objectVersion)
			}
			// NOTE: we are writing the RSA modulus content of the key
			if err = writeContent([]byte(*keybundle.Key.N), objectType, objectName, options.dir); err != nil {
				return err
			}
		case VaultTypeCertificate:
			certbundle, err := kvClient.GetCertificate(ctx, *vaultUrl, objectName, objectVersion)
			if err != nil {
				return wrapObjectTypeError(err, objectType, objectName, objectVersion)
			}
			if err = writeContent(*certbundle.Cer, objectType, objectName, options.dir); err != nil {
				return err
			}
		default:
			if err := errors.Errorf("Invalid vaultObjectTypes. Should be secret, key, or cert"); err != nil {
				return wrapObjectTypeError(err, objectType, objectName, objectVersion)
			}
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

func (adapter *KeyvaultFlexvolumeAdapter) getVaultURL() (vaultUrl *string, err error) {
	glog.V(2).Infof("subscriptionID: %s", adapter.options.subscriptionId)
	glog.V(2).Infof("vaultName: %s", adapter.options.vaultName)
	glog.V(2).Infof("resourceGroup: %s", adapter.options.resourceGroup)

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
