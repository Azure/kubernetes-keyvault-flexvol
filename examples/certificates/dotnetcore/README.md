#.netcore Examples#

These examples were ran using .net core version 2.2 as a way to read the certificate secrets and convert them into X509Certificate2 objects. You can run this example locally using .net core or by bundling this code into a pod alongside your K8s service.

#How to run the example alongside you Key Vault Flex Vol#
1. Ensure that you have set up the managed identity for your K8s cluster.
2. Apply the needed KeyVault FlexVol pod
3. Create a service or container that mounts the Key Vault Flex Vol.
4. Apply the .net core example pod to the environment.

#How to build and run the example locally:#
1. Download and install .netcore 2.2 SDK
2. Clone the enlistment or copy the code.
3. Build by running the command 'dotnet build'
4. Run by running the command 'dotnet run'
5. You'll be prompted for the volume/directory path along with the secret name.

#How to create a local test secret file:#
1. Ensure that you created the certificate in Azure Key Vault.
2. Ensure that you have installed the Azure CLI tools.
3. Open bash or powershell and login with 'az login' command. You might also need to set the default subscription.
4. Get the secret via one of the following commands:
    Bash:  'az keyvault secret show --vault-name <vaultname> --name <certificatename>' and the secret will show under 'value' in the json blob response.
    Powershell: '$secret = Get-AzureKeyVaultSecret -VaultName <vaultname> -Name <certificatename>' then you can enter '$secret.SecretValueText' to see the contents you need.
5. Copy the value to an empty file with no file extension (similarly to how the KV Flex Vol pod).

#Notes:#
1. The certificate was added to Azure KeyVault as a cert, but is referenced in our deployment yaml as a secret.
2. The certificate used in this example was self signed and with a password. I am able to read the cert secret without the password. However, you might need the password if you want to install the cert on the machine.