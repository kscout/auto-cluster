#!/usr/bin/env bash
# openshift-install-create-config.yaml.sh - Echos an openshift-install install configuration
#
# USAGE
#
#    openshift-install-create-config.yaml.sh CLUSTER_NAME
#
# ARGUMENTS
#
#    CLUSTER_NAME    Name of cluster to create
#
# REQUIREMENTS
#
#    Expects a pull-secret file to be adjacent to the script.
#
# CONFIGURATION
#
#    Environment variables are used to configure the script:
#
#    AUTO_CLUSTER_PULL_SECRET_PATH    Path to pull-secret file
#
#?

function die() {
    echo "Error: $@" >&2
    exit 1
} 

if [ -z "$1" ]; then
    die "CLUSTER_NAME argument required"
fi

if [ -z "$AUTO_CLUSTER_PULL_SECRET_PATH" ]; then
    AUTO_CLUSTER_PULL_SECRET_PATH=$(realpath ./pull-secret)
fi

if [ ! -f "$AUTO_CLUSTER_PULL_SECRET_PATH" ]; then
    die "$AUTO_CLUSTER_PULL_SECRET_PATH file not found"
fi

cat <<EOF
apiVersion: v1
baseDomain: devcluster.openshift.com
compute:
- hyperthreading: Enabled
  name: worker
  platform: {}
  replicas: 3
controlPlane:
  hyperthreading: Enabled
  name: master
  platform: {}
  replicas: 3
metadata:
  creationTimestamp: null
  name: "$1"
networking:
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  machineCIDR: 10.0.0.0/16
  networkType: OpenShiftSDN
  serviceNetwork:
  - 172.30.0.0/16
platform:
  aws:
    region: us-east-1
pullSecret: '$(cat $AUTO_CLUSTER_PULL_SECRET_PATH)'
EOF
