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
	"regexp"
	"strings"

	kv "github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
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
		glog.V(2).Infof("%s", options.tenantID)
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
				return sanitisedError(err, objectType, objectName, objectVersion)
			}
			if err = ioutil.WriteFile(fileName, []byte(*secret.Value), permission); err != nil {
				return errors.Wrapf(err, "azure KeyVault failed to write secret %s to %s", objectName, fileName)
			}
		case VaultTypeKey:
			keybundle, err := kvClient.GetKey(ctx, *vaultURL, objectName, objectVersion)
			if err != nil {
				return sanitisedError(err, objectType, objectName, objectVersion)
			}
			// NOTE: we are writing the RSA modulus content of the key
			if err = ioutil.WriteFile(fileName, []byte(*keybundle.Key.N), permission); err != nil {
				return errors.Wrapf(err, "azure KeyVault failed to write key %s to %s", objectName, fileName)
			}
		case VaultTypeCertificate:
			certbundle, err := kvClient.GetCertificate(ctx, *vaultURL, objectName, objectVersion)
			if err != nil {
				return sanitisedError(err, objectType, objectName, objectVersion)
			}
			if err = ioutil.WriteFile(fileName, *certbundle.Cer, permission); err != nil {
				return errors.Wrapf(err, "azure KeyVault failed to write certificate %s to %s", objectName, fileName)
			}
		default:
			err = errors.Errorf("Invalid vaultObjectTypes. Should be secret, key, or cert")
			return sanitisedError(err, objectType, objectName, objectVersion)
		}
		glog.V(0).Infof("azure KeyVault wrote %s %s at %s", objectType, objectName, fileName)
	}
	return nil
}

func (adapter *KeyvaultFlexvolumeAdapter) initializeKvClient() (*kv.BaseClient, error) {
	kvClient := kv.New()
	options := adapter.options

	token, err := GetKeyvaultToken(AuthGrantType(), options.cloudName, options.tenantID, options.usePodIdentity, options.useVmManagedIdentity, options.vmManagedIdentityClientID, options.aADClientSecret, options.aADClientID, options.podName, options.podNamespace)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get key vault token")
	}

	kvClient.Authorizer = token
	return &kvClient, nil
}

// azure-sdk-for-go returns some errors with \r\n in the body
// kubernetes errors out with "invalid character '\r' in string literal", if we don't sanitise it first
func sanitisedError(err error, objectType string, objectName string, objectVersion string) error {
	sanitisedErr := strings.Replace(err.Error(), "\\", " ", -1)
	return fmt.Errorf("failed to get objectType:%s, objectName:%s, objectVersion:%s %s", objectType, objectName, objectVersion, sanitisedErr)
}

func (adapter *KeyvaultFlexvolumeAdapter) getVaultURL() (vaultURL *string, err error) {
	// See docs for validation spec: https://docs.microsoft.com/en-us/azure/key-vault/about-keys-secrets-and-certificates#objects-identifiers-and-versioning
	if match, _ := regexp.MatchString("[-a-zA-Z0-9]{3,24}", adapter.options.vaultName); !match {
		return nil, errors.Errorf("Invalid vault name: %q, must match [-a-zA-Z0-9]{3,24}")
	}
	vaultDnsSuffix, err := GetVaultDNSSuffix(adapter.options.cloudName)
	if err != nil {
		return nil, err
	}

	vaultDnsSuffixValue := *vaultDnsSuffix

	vaultUri := "https://" + adapter.options.vaultName + "." + vaultDnsSuffixValue + "/"
	return &vaultUri, nil
}

func GetVaultDNSSuffix(cloudName string) (vaultTld *string, err error) {
	environment, err := ParseAzureEnvironment(cloudName)

	if err != nil {
		return nil, err
	}

	return &environment.KeyVaultDNSSuffix, nil
}
