#!/usr/bin/env bash
# migrate-cluster.sh - Migrate resources from one OpenShift cluster to another
#
# USAGE
#
#    migrate-cluster.sh -s STATE_DIR -f FROM -t TO -n NS
#
# OPTIONS
#
#    -s STATE_DIR    State directory
#    -f FROM         Name of cluster to migrate resources from
#    -t TO           Name of cluster ro migrate resources to
#    -n NS           Namespace to migrate clster from / to
#
# BEHAVIOR
#
#    Migrates subset of resources defined in the migrate_types variable.
#?

# Configuration
migrate_types=imagestream,configmap,secret,deploymentconfig,deployment,statefulset,service,ingress,routes,template,pv,pvc

# Helpers
prog_dir=$(realpath $(dirname "$0"))

function die() {
    echo "Error: $@" >&2
    exit 1
}

function bold() {
    echo "$(tput bold)$@$(tput sgr0)"
}

# Options
while getopts "s:f:t:n:" opt; do
    case "$opt" in
	s) state_dir="$OPTARG" ;;
	f) from="$OPTARG" ;;
	t) to="$OPTARG" ;;
	n) ns="$OPTARG" ;;
	?) die "Unknown option" ;;
    esac
done

if [ -z "$state_dir" ]; then
    die "-s STATE_DIR option required"
fi

if [ ! -d "$state_dir" ]; then
    die "-s STATE_DIR directory does not exist"
fi

if [ -z "$from" ]; then
    die "-f FROM option required"
fi

if [ ! -d "$state_dir/$from" ]; then
    die "Directory for $from cluster does not exist"
fi

if [ -z "$to" ]; then
    die "-s TO option required"
fi

if [ ! -d "$state_dir/$to" ]; then
    die "Directory from $to cluster does not exist"
fi

if [ -z "$ns" ]; then
    die "-n NS option required"
fi

# Try authenticating with each cluster
bold "Testing authentication"

from_kubeconfig="$state_dir/$from/auth/kubeconfig"
to_kubeconfig="$state_dir/$to/auth/kubeconfig"

if ! KUBECONFIG="$from_kubeconfig" oc get pods; then
    die "Failed to test authentication to $from cluster"
fi

if ! KUBECONFIG="$to_kubeconfig" oc get pods; then
    die "Failed to test authentication to $to cluster"
fi

# Migrate
bold "Exporting resources from $from cluster"

from_f="/tmp/from-$from.json"

if ! KUBECONFIG="$from_kubeconfig" oc get -n "$ns" "$migrate_types" -o json > "$from_f"; then
    rm "$from_f" || true
    die "Failed to export resources from $from cluster"
fi

bold "Importing resources to $to cluster"

if ! KUBECONFIG="$to_kubeconfig" oc get ns | grep "$ns"; then
    if ! KUBECONFIG="$to_kubeconfig" oc new-project "$ns"; then
	die "Failed to create new namespace $ns on $to cluster"
    fi
fi

if ! KUBECONFIG="$to_kubeconfig" oc apply -n "$ns" -f "$from_f"; then
    rm "$from_f" || true
    die "Failed to import resources to $to cluster"
fi

while read -r dc_name; do
    KUBECONFIG="$to_kubeconfig" oc rollout latest "dc/$dc_name"
done <<< $(KUBECONFIG="$to_kubeconfig" oc get dc -o=custom-columns=Name:.metadata.name | tail -n +2)

if ! rm "$from_f"; then
    die "Failed to delete export file $from_f"
fi

bold "Done"
