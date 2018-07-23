# Kubernetes-KeyVault-FlexVolume #

Azure Key Vault FlexVolume for Kubernetes - Integrates Azure Key Vault with Kubernetes via a FlexVolume.  

With the Azure Key Vault FlexVolume, developers can access application-specific secrets, keys, and certs stored in Azure Key Vault directly from their pods.

## **Project Status**: Alpha

## Design

The detailed design of this solution:

- [Concept](/docs/concept.md)

## How to use ##

### Prerequisites: ### 

💡 Make sure you have a Kubernetes cluster

### Install the KeyVault Flexvolume ###

#### OPTION 1 - ACS-Engine addon ####

Follow [this](https://github.com/Azure/acs-engine/blob/master/examples/addons/keyvault-flexvolume/README.md) to use acs-engine to create a new Kubernetes cluster with the Azure Key Vault FlexVolume already deployed.

#### OPTION 2 - Manually ####
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

The KeyVault FlexVolume offers two modes for accessing a Key Vault instance: Service Principal and Pod Idenity.

#### OPTION 1 - Service Principal ####

Add your service principal credentials as a Kubernetes secrets accessible by the KeyVault FlexVolume driver.

```bash
kubectl create secret generic kvcreds --from-literal clientid=<CLIENTID> --from-literal clientsecret=<CLIENTSECRET> --type="azure/kv”
```

Ensure this service principal has all the required permissions to access content in your key vault instance. 
If not, you can run the following using the Azure cli:

```bash
az keyvault set-policy -n $KV_NAME --key-permissions get list --spn <YOUR SPN CLIENT ID>
az keyvault set-policy -n $KV_NAME --secret-permissions get list --spn <YOUR SPN CLIENT ID>
az keyvault set-policy -n $KV_NAME --certificate-permissions get list --spn <YOUR SPN CLIENT ID>
```

Fill in the missing pieces in your deployment like [this](https://github.com/Azure/kubernetes-keyvault-flexvol/blob/master/deployment/nginx-flex-kv.yaml), make sure to:

1. reference the service principal kubernetes secret created in the previous step
```yaml
secretRef:
  name: kvcreds
```
2. pass in properties for the Key Vault instance to the flexvolume driver.

|Name|Required|Description|Default Value|
|---|---|---|---|
|usePodIdentity|no|specify access mode: service principal or pod identity|"false"|
|keyvaultname|yes|name of key vault instance|""|
|keyvaultobjectname|yes|name of key vault object to access|""|
|keyvaultobjecttype|yes|key vault object type: secret, key, cert|""|
|keyvaultobjectversion|yes|key vault object version|""|
|resourcegroup|yes|name of resource group containing key vault instance|""|
|subscriptionid|yes|name of subscription containing key vault instance|""|
|tenantid|yes|name of tenant containing key vault instance|""|

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
        usePodIdentity: "false"
        keyvaultname: "testkeyvault"
        keyvaultobjectname: "testsecret"
        keyvaultobjecttype: secret # OPTIONS: secret, key, cert
        keyvaultobjectversion: "testversion"
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

1. Deploy pod identity to your cluster
    Follow [these steps](https://github.com/Azure/aad-pod-identity#deploy-the-azure-aad-identity-infra) to install pod identity.

2. Create User Azure Identity 

    Create an Azure User Identity with the following command. 
    Get `clientId` and `id` from the output. 
    ```
    az identity create -g <resourcegroup> -n <idname>
    ```

3. Assign permissions to new identity
    Ensure your Azure user identity has all the required permissions to access content in your key vault instance. 
    If not, you can run the following using the Azure cli:

    ```bash
    az keyvault set-policy -n $KV_NAME --key-permissions get list --spn <YOUR AZURE USER IDENTITY CLIENT ID>
    az keyvault set-policy -n $KV_NAME --secret-permissions get list --spn <YOUR AZURE USER IDENTITY CLIENT ID>
    az keyvault set-policy -n $KV_NAME --certificate-permissions get list --spn <YOUR AZURE USER IDENTITY CLIENT ID>
    ```

4. Add a new `AzureIdentity` for the new identity to your cluster

    Edit and save this as aadpodidentity.yaml

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

5. Add a new `AzureIdentityBinding` for the nw identity to your cluster

    Edit and save this as aadpodidentitybinding.yaml
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
        app: nginx-flex-kv-podid
        aadpodidbinding: "NAME OF the AzureIdentityBinding SELECTOR"
    ```

    b. make sure to update `usePodIdentity` to `true`
    ```yaml
    usePodIdentity: "true"
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
