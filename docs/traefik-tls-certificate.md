# Using a keyvault certificate to setup SSL on traefik

The following assumes that you have an Azure Keyvault, and flexvol setup on your kubernetes cluster.
For the purpose of this documentation I have created a passwordless self-signed certificate in keyvault from azurce cli : 

```console
az keyvault certificate create --vault-name myvault -n cert1 -p "$(az keyvault certificate get-default-policy -o json)"
```

The yaml files configure a Traefik ingress controller, with a certificate from keyvault as default TLS certificate.

Here is a breakdown of the sections of yaml that matters : 

## Volumes

```yaml
- name: ssl
    emptyDir: {}
- name: certs
    flexVolume:
        driver: "azure/kv"
        options:
        keyvaultname: "clustervault1119"
        keyvaultobjectname: "cert1" 
        keyvaultobjecttype: "secret"
        resourcegroup: "aks-traefik-flexvol"
        subscriptionid: "SUB-ID"
        tenantid: "TENANT-ID"
        usepodidentity: "true" # the pod identity needs 
```

- `certs`:  the flexVolume that fetches the certificate. We download it as a `secret` to retrieve both the private and public part required for setting up TLS. Note that we use pod identity here. The Azure Identity bound to my Traefik pod (ref [identity.yaml](identity.yaml)) needs to have the appropriate rights to download the secret associated with the certificate.
- `ssl` : An `emptyDir` volume that translates to a tmpfs mount accessible only by this pod. We will use it to store the converted format of the certificate. 

## Containers

### initContainer

When we download the certificate from keyvault as a secret, to retrieve the private key as well as the public part, the file is in `PKCS12` format, and base64 encoded.
Traefik (and most ingresses), requires the certificate as PEM + private key pair format. We need to convert our PKCS12 file before starting Traefik. This is what the initContainer is doing :

```yaml
initContainers:
    - image: tannerfe/alpine-openssl
    name: convert-certs
    command: ['sh', '-c', 'cat /certs/cert1 | base64 -d - > /ssl/certificate.pfx &&
    openssl pkcs12 -in /ssl/certificate.pfx -out /ssl/certificate.pem -nodes -passin pass:"" &&
    openssl pkey -in /ssl/certificate.pem -out /ssl/certificate.key']
    volumeMounts:
    - name: certs
        mountPath: /certs
        readOnly: true
    - name: ssl
        mountPath: /ssl 
```

We use an alpine based openssl container. We mount the `certs` volume which is our readOnly flexvolume with the our PKCS12 certificate from KeyVault. We need to convert and save the certificate: we write to the `ssl` volume. It is this same `ssl` volume that we will mount in the Traefik ingress container to provide the PEM certificate and the private key.  
The first line of the `command` is reading the flexvolume certificate file, and decoding it from base64:

```console
sh -c cat /certs/cert1 | base64 -d - > /ssl/certificate.pfx
```

Converting the `PFX` to `PEM` :

```console
openssl pkcs12 -in /ssl/certificate.pfx -out /ssl/certificate.pem -nodes -passin pass:""
```console

Note that we use a self-signed certificate with no password (`--passin pass:""`). This might be different in your setup. 
The outputted `PEM` file contains both the public and private keys in clear. 

The last part of the command is extracting the private key to a separate file :

```console
openssl pkey -in /ssl/certificate.pem -out /ssl/certificate.key
```

### Traefik container

In the Traefik container, we mount the `ssl` volume alongside Traefik config volume from the `configmap`.
The toml configuration refers to the converted PEM and Key files in the ssl volume to setup the default TLS certificate : 

```toml
[entryPoints.https]
    address = ":443"
    [entryPoints.https.tls]
    [entryPoints.https.tls.defaultCertificate]
        certFile = "/ssl/certificate.pem"
        keyFile = "/ssl/certificate.key"
```

Our ingresses that expose an HTTPS endpoint will use this default certificate.
You can of course extend this and setup additional TLS entrypoints in traefik with different certificates from Keyvault as needed.

## NOTE: 

The Helm charts for Traefik does not provide the necessary options (init-conainter, toml config overrides) and insists on using the Kubernetes Secret objects to setup the certificates. That is the reason for this custom template.
The nginx-ingress implementation doesn't provide any way (to my knowledge) to configure the certificate from file, only kubernetes secrets are supported (https://kubernetes.github.io/ingress-nginx/user-guide/tls/).