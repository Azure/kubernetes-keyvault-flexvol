FROM alpine:3.11

WORKDIR /bin

RUN apk add --no-cache bash
ADD ./kv /bin/kv
ADD ./azurekeyvault-flexvolume /bin/azurekeyvault-flexvolume
RUN chmod a+x /bin/kv
RUN chmod a+x /bin/azurekeyvault-flexvolume
ADD ./install.sh /bin/install_kv_flexvol.sh

# Adds a user and group with name of flexvol to the OS for the application to run under (least privilege)
RUN addgroup -S flexvol && adduser -S flexvol -G flexvol
# Fetches updates, installs any updates to the OS required and removes the cache files to keep the image small.
RUN apk update && apk upgrade && rm -rf /var/cache/apk/*
# Gives owner permissions to the directory flexvol will run the install from
RUN chown -R flexvol:flexvol /bin
# Runs any subsequent command under the created user and not as root
USER flexvol

ENTRYPOINT ["/bin/install_kv_flexvol.sh"] 