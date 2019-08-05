g #!/usr/bin/env bash
# install-helm-chart.sh - Install a Helm chart on a cluster
#
# USAGE
#
#    install-helm-chart.sh.sh -s STATE_DIR -c CLUSTER -n NS CHART
#
# OPTIONS
#
#    -s STATE_DIR    State directory
#    -c CLUSTER      Name of cluster to install Helm chart
#    -n NS           Namespace to migrate clster from / to
#
# ARGUMENTS
#
#    CHART    Git URI of Helm chart to install.
#
# BEHAVIOR
#
#    Installs a Helm chart on a cluster.
#?

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
	c) cluster_name="$OPTARG" ;;
	n) ns="$OPTARG" ;;
	?) die "Unknown option" ;;
    esac
done

shift $((OPTIND-1))

if [ -z "$state_dir" ]; then
    die "-s STATE_DIR option required"
fi

if [ ! -d "$state_dir" ]; then
    die "-s STATE_DIR directory does not exist"
fi

if [ -z "$cluster_name" ]; then
    die "-c CLUSTER option required"
fi

if [ ! -d "$state_dir/$cluster_name" ]; then
    die "Directory from $cluster_name cluster does not exist"
fi

if [ -z "$ns" ]; then
    die "-n NS option required"
fi

# Arugments
chart="$1"
if [ -z "$chart" ]; then
    die "CHART argument required"
fi

# Try authenticating with each cluster
bold "Testing authentication"

export KUBECONFIG="$state_dir/$cluster_name/auth/kubeconfig"

if ! oc get pods; then
    die "Failed to test authentication to $cluster_name cluster"
fi

# Download Helm chart
helm_chart_dir=$(basename "$chart")
helm_chart_dir=/tmp/${helm_chart_dir%.*}

bold "Downloading Helm chart"

if ! git clone "$chart" "$helm_chart_dir"; then
    die "Failed to download Helm chart"
fi

# Install
bold "Install Helm chart"

if ! helm template "$helm_chart_dir" | oc apply -f -; then
    die "Failed to install \"$chart\" Chart on $cluster_name"
fi

bold "Done"
