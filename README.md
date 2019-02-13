# Kubernetes-KeyVault-FlexVolume #

Key Vault FlexVolume for Kubernetes - Integrates Key Management Systems with Kubernetes via a FlexVolume.  

With the Key Vault FlexVolume, developers can mount multiple secrets, keys, and certs stored in Key Management Systems into their pods as a volume. Once the Volume is attached, the data in it is mounted into the container's file system. 

[![CircleCI](https://circleci.com/gh/Azure/kubernetes-keyvault-flexvol/tree/master.svg?style=svg)](https://circleci.com/gh/Azure/kubernetes-keyvault-flexvol/tree/master)

## Supported Providers
* Azure Key Vault

> 💡 NOTE: To enable encryption at rest of Kubernetes data in etcd using Azure Key Vault, use [Kubernetes KMS plugin for Azure Key Vault](https://github.com/Azure/kubernetes-kms).

## Design

The detailed design of this solution:

- [Concept](/docs/concept.md)

## How to use ##

### Prerequisites: ### 

💡 Make sure you have a Kubernetes cluster

### Install the KeyVault Flexvolume ###

#### OPTION 1 - AKS-Engine addon ####

Follow [this](https://github.com/Azure/aks-engine/blob/master/examples/addons/keyvault-flexvolume/README.md) to use aks-engine to create a new Kubernetes cluster with the Key Vault FlexVolume already deployed.

#### OPTION 2 - AKS (Azure Kubernetes Service) Manually ####

```bash

kubectl create -f https://raw.githubusercontent.com/Azure/kubernetes-keyvault-flexvol/master/deployment/kv-flexvol-installer.yaml
```
To validate the installer is running as expected, run the following commands:

```bash
kubectl get pods -n kv
```

You should see the keyvault flexvolume installer pods running on each agent node:

```bash
keyvault-flexvolume-f7bx8   1/1       Running   0          3m
keyvault-flexvolume-rcxbl   1/1       Running   0          3m
keyvault-flexvolume-z6jm6   1/1       Running   0          3m
```
### Use the KeyVault FlexVolume ###

The KeyVault FlexVolume offers two modes for accessing a Key Vault instance: Service Principal and Pod Identity.

#### OPTION 1 - Service Principal ####

Add your service principal credentials as a Kubernetes secrets accessible by the KeyVault FlexVolume driver.

```bash
kubectl create secret generic kvcreds --from-literal clientid=<CLIENTID> --from-literal clientsecret=<CLIENTSECRET> --type=azure/kv
```

Ensure this service principal has all the required permissions to access content in your key vault instance. 
If not, you can run the following using the Azure cli:

```bash
# Assign Reader Role to the service principal for your keyvault
az role assignment create --role Reader --assignee <principalid> --scope /subscriptions/<subscriptionid>/resourcegroups/<resourcegroup>/providers/Microsoft.KeyVault/vaults/<keyvaultname>

az keyvault set-policy -n $KV_NAME --key-permissions get --spn <YOUR SPN CLIENT ID>
az keyvault set-policy -n $KV_NAME --secret-permissions get --spn <YOUR SPN CLIENT ID>
az keyvault set-policy -n $KV_NAME --certificate-permissions get --spn <YOUR SPN CLIENT ID>
```

Fill in the missing pieces in [this](https://github.com/Azure/kubernetes-keyvault-flexvol/blob/master/deployment/nginx-flex-kv.yaml) deployment for your own deployment, make sure to:

1. reference the service principal kubernetes secret created in the previous step
```yaml
secretRef:
  name: kvcreds
```
2. pass in properties for the Key Vault instance to the flexvolume driver.

|Name|Required|Description|Default Value|
|---|---|---|---|
|usepodidentity|no|specify access mode: service principal or pod identity|"false"|
|keyvaultname|yes|name of KeyVault instance|""|
|keyvaultobjectnames|yes|names of KeyVault objects to access|""|
|keyvaultobjecttypes|yes|types of KeyVault objects: secret, key or cert|""|
|keyvaultobjectversions|no|versions of KeyVault objects, if not provided, will use latest|""|
|resourcegroup|yes|name of resource group containing key vault instance|""|
|subscriptionid|yes|name of subscription containing key vault instance|""|
|tenantid|yes|name of tenant containing key vault instance|""|

keyvaultobjectnames, keyvaultobjecttypes and keyvaultobjectversions are semi-colon (;) separated.

3. Specify mount path of flexvolume to mount key vault objects
```yaml
volumeMounts:
    - name: test
      mountPath: /kvmnt
      readOnly: true
```

Example of an nginx pod accessing a secret from a key vault instance:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx-flex-kv
spec:
  containers:
  - name: nginx-flex-kv
    image: nginx
    volumeMounts:
    - name: test
      mountPath: /kvmnt
      readOnly: true
  volumes:
  - name: test
    flexVolume:
      driver: "azure/kv"
      secretRef:
        name: kvcreds # mounting point to the pod
      options:
        usepodidentity: "false"
        keyvaultname: "testkeyvault"
        keyvaultobjectnames: "testsecret"
        keyvaultobjecttypes: secret # OPTIONS: secret, key, cert
        keyvaultobjectversions: "testversion"
        resourcegroup: "testresourcegroup"
        subscriptionid: "testsub"
        tenantid: "testtenant"
```

Deploy your app

```bash
kubectl create -f deployment/nginx-flex-kv.yaml
```

Validate the pod has access to the secret from key vault:

```bash
kubectl exec -it nginx-flex-kv cat /kvmnt/testsecret
testvalue
```

#### OPTION 2 - Pod identity ####

##### Prerequisites: #####

💡 Make sure you have installed pod identity to your Kubernetes cluster

1. Deploy pod identity components to your cluster
    Follow [these steps](https://github.com/Azure/aad-pod-identity#deploy-the-azure-aad-identity-infra) to install pod identity.

2. Create an Azure User Identity 

    Create an Azure User Identity with the following command. 
    Get `clientId` and `id` from the output. 
    ```
    az identity create -g <resourcegroup> -n <idname>
    ```

3. Assign permissions to new identity
    Ensure your Azure user identity has all the required permissions to read the keyvault instance and to access content within your key vault instance. 
    If not, you can run the following using the Azure cli:

    ```bash
    # Assign Reader Role to new Identity for your keyvault
    az role assignment create --role Reader --assignee <principalid> --scope /subscriptions/<subscriptionid>/resourcegroups/<resourcegroup>/providers/Microsoft.KeyVault/vaults/<keyvaultname>

    # set policy to access keys in your keyvault
    az keyvault set-policy -n $KV_NAME --key-permissions get --spn <YOUR AZURE USER IDENTITY CLIENT ID>
    # set policy to access secrets in your keyvault
    az keyvault set-policy -n $KV_NAME --secret-permissions get --spn <YOUR AZURE USER IDENTITY CLIENT ID>
    # set policy to access certs in your keyvault
    az keyvault set-policy -n $KV_NAME --certificate-permissions get --spn <YOUR AZURE USER IDENTITY CLIENT ID>
    ```

4. Add a new `AzureIdentity` for the new identity to your cluster

    Edit and save this as `aadpodidentity.yaml`

    Set `type: 0` for Managed Service Identity; `type: 1` for Service Principal
    In this case, we are using managed service identity, `type: 0`.
    Create a new name for the AzureIdentity. 
    Set `ResourceID` to `id` of the Azure User Identity created from the previous step.

    ```yaml
    apiVersion: "aadpodidentity.k8s.io/v1"
    kind: AzureIdentity
    metadata:
     name: <any-name>
    spec:
     type: 0
     ResourceID: /subscriptions/<subid>/resourcegroups/<resourcegroup>/providers/Microsoft.ManagedIdentity/userAssignedIdentities/<idname>
     ClientID: <clientid>
    ```

    ```bash
    kubectl create -f aadpodidentity.yaml
    ```

5. Add a new `AzureIdentityBinding` for the new Azure identity to your cluster

    Edit and save this as `aadpodidentitybinding.yaml`
    ```yaml
    apiVersion: "aadpodidentity.k8s.io/v1"
    kind: AzureIdentityBinding
    metadata:
     name: demo1-azure-identity-binding
    spec:
     AzureIdentity: <name_of_AzureIdentity_created_from_previous_step>
     Selector: <label value to match in your app>
    ``` 

    ```
    kubectl create -f aadpodidentitybinding.yaml
    ```

6. Add the following to your deployment yaml, like [deployment/nginx-flex-kv-podidentity.yaml](https://github.com/Azure/kubernetes-keyvault-flexvol/blob/master/deployment/nginx-flex-kv-podidentity.yaml):

    a. Include the `aadpodidbinding` label matching the `Selector` value set in the previous step so that this pod will be assigned an identity
    ```yaml
    metadata:
    labels:
        aadpodidbinding: "NAME OF the AzureIdentityBinding SELECTOR"
    ```

    b. make sure to update `usepodidentity` to `true`
    ```yaml
    usepodidentity: "true"
    ```

7. Deploy your app

    ```bash
    kubectl create -f deployment/nginx-flex-kv-podidentity.yaml
    ```

8. Validate the pod has access to the secret from key vault:

    ```bash
    kubectl exec -it nginx-flex-kv-podid cat /kvmnt/testsecret
    testvalue
    ```

### Specific use cases ###

- [A detailed example for using a KeyVault certificate to setup an SSL entrypoint with Traefik](docs/traefik-tls-certificate.md)
- [An example of reading KeyVault certificates as secrets with .netcore](examples/certificates/dotnetcore/README.md)

# About KeyVault 

The Key Vault FlexVolume interacts with keyvault objects by using the keyvault API. If you need to understand the difference between Keys, Secrets and Certificate objects, we recommend that you start by reading the thorough documentation available on Keyvault : [About keys, secrets, and certificates](https://docs.microsoft.com/en-us/azure/key-vault/about-keys-secrets-and-certificates)

## More about Certificates

It is important to understand how a certificate is structured in keyvault.
As mentioned in the REST API docs [here](https://docs.microsoft.com/en-us/azure/key-vault/certificate-scenarios#certificates-are-complex-objects) and [here](https://docs.microsoft.com/en-us/azure/key-vault/about-keys-secrets-and-certificates#composition-of-a-certificate), Azure Key Vault (AKV) represents a given X.509 certificate via three interrelated resources: an AKV-certificate, an AKV-key, and an AKV-secret. All three will share the same name and the same version and can be fetched independently.

* The AKV-certificate provides the public key and certificate metadata. Specifying `cert` in `keyvaultobjecttypes` will fetch the public key and certificate metadata.
* The AKV-key provides the private key of the X.509 certificate. It can be useful for performing cryptographic operations such as signing if the corresponding certificate was marked as non-exportable. Specifying `key` in `keyvaultobjecttypes` will fetch the private key of the certificate if its policy allows for private key exporting.
* The AKV-secret provides a way to export the full X.509 certificate, including its private key (if its policy allows for private key exporting). Specifying `secret` in `keyvaultobjecttypes` will fetch the base64-encoded certificate bundle.


# Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
