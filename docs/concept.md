# Basic Requirements

1. Drive Azure KeyVault typed objects (secrets, certificates and keys) into pods via projection (similar to secrets/config maps volumes). 
2. Enable low footprint integration, users shouldn't configure pods differently (example side cars) to allow this integration. 
3. Enable role control and authorization assertion on pod level.

## Components:

### Azure KeyVault Volume Driver

Binary deployed on all nodes, this binary implements flex vol interfaces. The binary downloads the keyvault objects and project them 
into pods. The volume driver has two modes of operation:

1. Stand Alone

The driver uses a secret (contains Service Principal username + password) via FlexVol `SecretRef` to authenticate itself on
KeyVault

2. Integrated with Pod Identity

The driver uses pods own identity via [aad pod identity](https://github.com/Azure/aad-pod-identity/) to authenticate itself on KeyVault.

> in both cases, the user is required to set the correct permission via ARM roles on KeyVault.


## Spec

### Flex Vol Spec

The following fields are expected to be part of Flex Vol

1. Region: `required` (example: `centralus`)
2. Cloud Name: `required` (example: `azure` [default], Federal, Germany ..). The names are expected to match go-lang sdk cloud names)
3. KeyVault Name: `required` 
4. KeyVault Object Name: `required`
5. KeyVault Object Type: `required`
6. KeyVault Object Version: `optional` (`empty` == latest version)
7. Use integrated identity. `optional` (Default `false` (mode #1), true (mode #2))
8. Alias: `optional` by default keyvault objects are projected with thier name, alias is to override the filename.

##  Flows

> The flexvol is not expected to perform attach/detach instead it will do `mount` and `unmount` only.

### Stand Alone Mode

The flex vol driver will use the `secret.ServicePrincipalName` and `secret.ServicePrincipalPassword` with ADAL to acquire the 
object from KeyVault. Flexvol driver will create the file at `MNTPATH` as passed by kubernetes. 

> Note: the flexvol driver is expected to set the correct file permission on the object (TBD).


### Integrated With Pod Identity Mode

FlexVol (based on the `useIntegratedIdentity` spec parameter) will instead call pod identity `NMI` component with ADAL on a specific endpoint
offered only on host network. 

> spike: can we add more headers to this particular call to carry information such as pod name/namespace?

