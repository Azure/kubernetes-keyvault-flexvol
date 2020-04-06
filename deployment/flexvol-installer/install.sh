#!/bin/bash

set -eo pipefail

target_dir="${TARGET_DIR}"

if [[ -z "${target_dir}" ]];then
  echo "target dir is not set. please set TARGET_DIR env var"
  exit 1 # if not set this will put the pod in crash loop
fi

kv_vol_dir="${target_dir}/azure~kv"
mkdir -p ${kv_vol_dir}

#copy
cp /bin/kv ${kv_vol_dir}/kv
cp /bin/azurekeyvault-flexvolume ${kv_vol_dir}/azurekeyvault-flexvolume #script
cp /bin/jq-replacement.py ${kv_vol_dir}/jq-replacement.py


#https://github.com/kubernetes/kubernetes/issues/17182
# if we are running on kubernetes cluster as a daemon set we should
# not exit otherwise, container will restart and goes into crashloop (even if exit code is 0)
while true; do echo "install done, daemonset sleeping" && sleep 60; done

