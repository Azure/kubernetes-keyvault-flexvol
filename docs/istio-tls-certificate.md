# Using a keyvault certificate to setup an SSL entrypoint with Istio

## Prerequisites

The following guide presumes you have done the following:

* Provisioned an Azure KeyVault
* Provisioned a Kubernetes Cluster
* Installed the [Istio Service Mesh](https://istio.io/docs/setup/kubernetes/)
* Installed and configured the [kubernetes-keyvault-flexvol](https://github.com/Azure/kubernetes-keyvault-flexvol) project with a Service Principal
* The Azure CLI installed
* OpenSSL tools installed

Additionally, you'll need to confirm that you can make changes to the istio-ingressgateway in the `istio-system` namespace. We will be making changes to the canonical yaml that represents the ingress gateway in this guide for reasons explained later.

## Create and upload a self signed certificate

To demonstrate configuring Istio to work with an SSL certificates hosted in Keyvault, let's create a self signed certificate and private key. In a production setting, you will want a signed certificate from a trusted issuer. [Let's Encrypt](https://letsencrypt.org/) offers signed certificates for free, with the understanding that the certificates expire in [90 days](https://letsencrypt.org/2015/11/09/why-90-days.html).

To create a certificate and private key, run the following in your terminal, and fill out the information requested:

```bash
openssl req -x509 -newkey rsa:2048 -keyout certificate-private.key -out certificate.crt -days 365 -nodes
```

OpenSSL will create a certificate (certificate.crt) and a private key (certificate-private.key). We will be upload these to keyvault.

Keyvault expects that a certificate be uploaded with it's private key. To do this, we concatenate the certificate with the private key into a new file, certificate.includesprivatekey.pem:

```bash
cat certificate.crt certificate-private.key > certificate.includesprivatekey.pem
```

Then, we upload the certificate and private to keyvault:

```bash
az keyvault certificate import -n mycertificate --vault-name mykeyvault -f certificate.includesprivatekey.pem
```

```bash
az keyvault key import -n myprivatekey --vault-name mykeyvault -f certificate-private.key
```

## Configure the Istio Ingress Gateway

From here on, configuration is manual. As of Istio 1.0.6, Istio does not yet support configuring the ingress gateway to support external Key Management Services. Refer to the [sample ingress-gateway](./istio-tls-certificate/istio-ingressgateway.yaml) for how to configure the ingressgateway. Let's break this sample down.

To the ingress gateway, we add an extra read-only volume mount that refers to the `keyvault-certs` volume:

```yaml
        - mountPath: /etc/istio/keyvault-certs
          name: keyvault-certs
          readOnly: true
```

We also add an extra volume that is referred to by the volume mount above:

```yaml
      - name: keyvault-certs
        flexVolume:
          driver: "azure/kv"
          secretRef:
            kvcreds
          options:
            usepodidentity: "false"
            keyvaultname: "mykeyvault"
            keyvaultobjectnames: "myprivatekey;mycertificate"
            keyvaultobjecttypes: "key;cert"
            resourcegroup: "mykeyvaultgroup"
            subscriptionid: "azuresubscriptionid"
            tenantid: "azuretenantid"
```

In the above example, we declare a volume named `keyvault-certs`, and configure the keyvault volume driver with a secret [backed by a service principal](https://github.com/Azure/kubernetes-keyvault-flexvol#option-1---service-principal). We also configure the names of the private key and certificate, and the types we request from keyvault, along with the resource group of the keyvault, the subcription id, and the tenant id.

Next, we configure an Istio Gateway on port 443. [See the provided gateway sample](./istio-tls-certificate/istio-samplegateway.yaml) for a full example of a configured gateway:

```yaml
  - port:
      number: 443
      name: https
      protocol: HTTPS
    tls:
      mode: SIMPLE
      serverCertificate: /etc/istio/keyvault-certs/mycertificate
      privateKey: /etc/istio/keyvault-certs/myprivatekey
```

As we've configured the Ingress Gateway Deployment to read certificates from Keyvault, we can simply point the gatway's tls options to the certificate and private key pulled down from Keyvault.


