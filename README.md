# Key Vault FlexVolume

Seamlessly integrate your key management systems with Kubernetes.

Secrets, keys, and certificates in a key management system become a volume accessible to pods. Once the volume is mounted, its data is available directly in the container filesystem for your application.

[![CircleCI](https://circleci.com/gh/Azure/kubernetes-keyvault-flexvol/tree/master.svg?style=svg)](https://circleci.com/gh/Azure/kubernetes-keyvault-flexvol/tree/master)

## Contents

* [Getting Started](#getting-started)
* [Detailed Use Cases](#detailed-use-cases)
* [Design](#design)
* [About Key Vault](#about-key-vault)
* [About Certificates](#about-certificates)
* [Contributing](#contributing)
* [Code of Conduct](#code-of-conduct)

## Getting started

### Supported Providers

* Azure Key Vault

  > 💡 **NOTE**: To enable encryption at rest of Kubernetes data in `etcd`, use the [Kubernetes KMS plugin] for Azure Key Vault.

### Installing Key Vault FlexVolume

#### OPTION 1: New AKS Engine cluster

[AKS Engine] creates customized Kubernetes clusters on Azure.

Follow the AKS Engine [add-on documentation] to create a new Kubernetes cluster with Key Vault FlexVolume already deployed.

#### OPTION 2: Existing AKS cluster

Azure Kubernetes Service ([AKS]) creates managed, supported Kubernetes clusters on Azure.

Deploy Key Vault FlexVolume to your AKS cluster with this command:

```bash
kubectl create -f https://raw.githubusercontent.com/Azure/kubernetes-keyvault-flexvol/master/deployment/kv-flexvol-installer.yaml
```

 To validate Key Vault FlexVolume is running as expected, run the following command:

 ```bash
 kubectl get pods -n kv
 ```

 The output should show `keyvault-flexvolume` pods running on each agent node:

  ```bash
  NAME                        READY     STATUS    RESTARTS   AGE
  keyvault-flexvolume-f7bx8   1/1       Running   0          3m
  keyvault-flexvolume-rcxbl   1/1       Running   0          3m
  keyvault-flexvolume-z6jm6   1/1       Running   0          3m
  ```

### Using Key Vault FlexVolume

Key Vault FlexVolume offers two modes for accessing a Key Vault instance: [Service Principal] and [Pod Identity].

#### OPTION 1: Service Principal

Add your service principal credentials as Kubernetes secrets accessible by the Key Vault FlexVolume driver.

```bash
kubectl create secret generic kvcreds --from-literal clientid=<CLIENTID> --from-literal clientsecret=<CLIENTSECRET> --type=azure/kv
```

Ensure this service principal has all the required permissions to access content in your Key Vault instance.
If not, run the following [Azure CLI] commands:

```bash
# Assign Reader Role to the service principal for your keyvault
az role assignment create --role Reader --assignee <principalid> --scope /subscriptions/<subscriptionid>/resourcegroups/<resourcegroup>/providers/Microsoft.KeyVault/vaults/<keyvaultname>

az keyvault set-policy -n $KV_NAME --key-permissions get --spn <YOUR SPN CLIENT ID>
az keyvault set-policy -n $KV_NAME --secret-permissions get --spn <YOUR SPN CLIENT ID>
az keyvault set-policy -n $KV_NAME --certificate-permissions get --spn <YOUR SPN CLIENT ID>
```

Fill in the missing pieces in [this](https://github.com/Azure/kubernetes-keyvault-flexvol/blob/master/deployment/nginx-flex-kv.yaml) deployment for your own deployment. Make sure to:

1. Reference the service principal Kubernetes secret created in the previous step

    ```yaml
    secretRef:
      name: kvcreds
    ```

2. Pass in properties for the Key Vault instance to the FlexVolume driver.

    |Name|Required|Description|Default Value|
    |---|---|---|---|
    |usepodidentity|no|specify access mode: service principal or pod identity|"false"|
    |keyvaultname|yes|name of Key Vault instance|""|
    |keyvaultobjectnames|yes|names of Key Vault objects to access|""|
    |keyvaultobjectaliases|no|filenames to use when writing the objects|keyvaultobjectnames|
    |keyvaultobjecttypes|yes|types of Key Vault objects: secret, key or cert|""|
    |keyvaultobjectversions|no|versions of Key Vault objects, if not provided, will use latest|""|
    |resourcegroup|required for version < v0.0.14|name of resource group containing Key Vault instance|""|
    |subscriptionid|required for version < v0.0.14|name of subscription containing Key Vault instance|""|
    |tenantid|yes|name of tenant containing Key Vault instance|""|

    Multiple values in the `keyvaultobjectnames`, `keyvaultobjecttypes` and `keyvaultobjectversions` properties should be separated with semicolons (`;`).

3. Specify mount path of flexvolume to mount key vault objects

    ```yaml
    volumeMounts:
       - name: test
          mountPath: /kvmnt
          readOnly: true
    ```

    Example of an nginx pod accessing a secret from a Key Vault instance:

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
            name: kvcreds                             # [OPTIONAL] not required if using Pod Identity
          options:
            usepodidentity: "false"                   # [OPTIONAL] if not provided, will default to "false"
            keyvaultname: "testkeyvault"              # [REQUIRED] the name of the KeyVault
            keyvaultobjectnames: "testsecret"         # [REQUIRED] list of KeyVault object names (semi-colon separated)
            keyvaultobjectaliases: "secret.json"      # [OPTIONAL] list of KeyVault object aliases
            keyvaultobjecttypes: secret               # [REQUIRED] list of KeyVault object types: secret, key, cert
            keyvaultobjectversions: "testversion"     # [OPTIONAL] list of KeyVault object versions (semi-colon separated), will get latest if empty
            resourcegroup: "testresourcegroup"        # [REQUIRED for version < v0.0.14] the resource group of the KeyVault
            subscriptionid: "testsub"                 # [REQUIRED for version < v0.0.14] the subscription ID of the KeyVault
            tenantid: "testtenant"                    # [REQUIRED] the tenant ID of the KeyVault
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

#### OPTION 2: Pod identity

💡 The basic steps to configure [AAD Pod Identity] are reproduced here, but please refer to that project's [README][aad-pod-id-README] for more detail.

1. Install AAD Pod Identity

   Run this command to create the `aad-pod-identity` deployment on an RBAC-enabled cluster:

   ```shell
   kubectl apply -f https://raw.githubusercontent.com/Azure/aad-pod-identity/master/deploy/infra/deployment-rbac.yaml
   ```

2. Assign Cluster SPN Role

   If the Service Principal used for the cluster was created separately (not automatically, as part of an AKS cluster's `MC_` resource group), assign it the "Managed Identity Operator" role:

   ```bash
   az role assignment create --role "Managed Identity Operator" --assignee <sp id> --scope <full id of the managed identity>
   ```

3. Create an Azure Identity

   Run this [Azure CLI] command, and take note of the `clientId` and `id` values it returns:

   ```shell
   az identity create -g <resourcegroup> -n <name> -o json
   ```

4. Assign Azure Identity Roles

   Ensure that your Azure Identity has the role assignments required to see your Key Vault instance and to access its content. Run the following Azure CLI commands to assign these roles if needed:

   ```bash
   # Assign Reader Role to new Identity for your Key Vault
   az role assignment create --role Reader --assignee <principalid> --scope /subscriptions/<subscriptionid>/resourcegroups/<resourcegroup>/providers/Microsoft.KeyVault/vaults/<keyvaultname>

   # set policy to access keys in your Key Vault
   az keyvault set-policy -n $KV_NAME --key-permissions get --spn <YOUR AZURE USER IDENTITY CLIENT ID>
   # set policy to access secrets in your Key Vault
   az keyvault set-policy -n $KV_NAME --secret-permissions get --spn <YOUR AZURE USER IDENTITY CLIENT ID>
   # set policy to access certs in your Key Vault
   az keyvault set-policy -n $KV_NAME --certificate-permissions get --spn <YOUR AZURE USER IDENTITY CLIENT ID>
   ```

5. Install the Azure Identity

   Save this Kubernetes manifest to a file named `aadpodidentity.yaml`:

   ```yaml
   apiVersion: "aadpodidentity.k8s.io/v1"
   kind: AzureIdentity
   metadata:
     name: <a-idname>
   spec:
     type: 0
     ResourceID: /subscriptions/<subid>/resourcegroups/<resourcegroup>/providers/Microsoft.ManagedIdentity/userAssignedIdentities/<name>
     ClientID: <clientId>
   ```

   Replace the placeholders with your user identity values. Set `type: 0` for user-assigned MSI.

   Finally, save your changes to the file, then create the `AzureIdentity` resource in your cluster:

   ```shell
   kubectl apply -f aadpodidentity.yaml
   ```

6. Install the Azure Identity Binding

    Save this Kubernetes manifest to a file named `aadpodidentitybinding.yaml`:

    ```yaml
    apiVersion: "aadpodidentity.k8s.io/v1"
    kind: AzureIdentityBinding
    metadata:
      name: demo1-azure-identity-binding
    spec:
      AzureIdentity: <a-idname>
      Selector: <label value to match>
    ```

   Replace the placeholders with your values. Ensure that the `AzureIdentity` name matches the one in `aadpodidentity.yaml`.

   Finally, save your changes to the file, then create the `AzureIdentityBinding` resource in your cluster:

   ```shell
   kubectl apply -f aadpodidentitybinding.yaml
   ```

7. Update the Application Deployment

    Your application manifest needs a couple of changes. Refer to the [nginx-flex-kv-podid] deployment as an example.

    a. Include the `aadpodidbinding` label to match the `Selector` value from the previous step:

    ```yaml
    metadata:
    labels:
        aadpodidbinding: "NAME OF the AzureIdentityBinding SELECTOR"
    ```

    b. Set `usepodidentity` to `true`:

    ```yaml
    usepodidentity: "true"
    ```

8. Deploy your app

    ```bash
    kubectl create -f deployment/nginx-flex-kv-podidentity.yaml
    ```

9. Validate the pod can access the secret from Key Vault:

    ```bash
    kubectl exec -it nginx-flex-kv-podid cat /kvmnt/testsecret
    testvalue
    ```

  > **NOTE**: When using the `Pod Identity` option mode, there may be some delay in obtaining the objects from Key Vault. During pod creation time, AAD Pod Identity needs to create the `AzureAssignedIdentity` for the pod based on the `AzureIdentity` and `AzureIdentityBinding` and retrieve the token for Key Vault. It is possible for the pod volume mount to fail during this time. If it does, the kubelet will keep retrying until after the token retrieval is complete and the mount succeeds.

## Detailed use cases

* Use Key Vault FlexVol to set up an [SSL entrypoint with Istio]
* Use Key Vault FlexVol to set up an [SSL entrypoint with Traefik]

## Design

To learn more about the design of Key Vault FlexVolume, see [Concept].

## About Key Vault

Key Vault FlexVolume interacts with Key Vault objects by using the [Key Vault API].

Azure Key Vault has thorough documentation available to help clarify the difference between [keys, secrets, and certificates].

## About Certificates

It is important to understand how a certificate is structured in Key Vault.

As mentioned in [Certificates are complex objects] and [Composition of a Certificate], Azure Key Vault (AKV) represents an X.509 certificate as three related resources:

* an AKV-certificate
* an AKV-key
* an AKV-secret

All three will share the same name and the same version and can be fetched independently.

* The AKV-certificate provides the public key and certificate metadata. Specifying `cert` in `keyvaultobjecttypes` will fetch the public key and certificate metadata.
* The AKV-key provides the private key of the X.509 certificate. It can be useful for performing cryptographic operations such as signing if the corresponding certificate was marked as non-exportable. Specifying `key` in `keyvaultobjecttypes` will fetch the private key of the certificate if its policy allows for private key exporting.
* The AKV-secret provides a way to export the full X.509 certificate, including its private key (if its policy allows for private key exporting). Specifying `secret` in `keyvaultobjecttypes` will fetch the base64-encoded certificate bundle.

## Contributing

The Key Vault FlexVolume project welcomes contributions and suggestions. Please see [CONTRIBUTING](CONTRIBUTING.md) for details.

## Code of conduct

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/). For more information, see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq) or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

[AAD Pod Identity]: https://github.com/Azure/aad-pod-identity
[aad-pod-id-README]: https://github.com/Azure/aad-pod-identity#readme
[add-on documentation]: https://github.com/Azure/aks-engine/blob/master/examples/addons/keyvault-flexvolume/README.md
[AKS]: https://azure.microsoft.com/services/kubernetes-service/
[AKS Engine]: https://github.com/Azure/aks-engine
[Azure CLI]: https://docs.microsoft.com/cli/azure/install-azure-cli
[Certificates are complex objects]: https://docs.microsoft.com/azure/key-vault/certificate-scenarios#certificates-are-complex-objects
[Composition of a Certificate]: https://docs.microsoft.com/azure/key-vault/about-keys-secrets-and-certificates#composition-of-a-certificate
[Concept]: /docs/concept.md
[keys, secrets, and certificates]: https://docs.microsoft.com/azure/key-vault/about-keys-secrets-and-certificates
[Key Vault API]: https://docs.microsoft.com/rest/api/keyvault/
[Kubernetes KMS plugin]: https://github.com/Azure/kubernetes-kms
[nginx-flex-kv-podid]: https://github.com/Azure/kubernetes-keyvault-flexvol/blob/master/deployment/nginx-flex-kv-podidentity.yaml
[Pod Identity]: #option-2-pod-identity
[Service Principal]: #option-1-service-principal
[SSL entrypoint with Istio]: docs/istio-tls-certificate.md
[SSL entrypoint with Traefik]: docs/traefik-tls-certificate.md
