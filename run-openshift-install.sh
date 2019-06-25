#!/usr/bin/env bash
# run-openshift-install.sh - Runs openshift-install on behalf of the Go program
#
# USAGE
#
#    run-openshift-install.sh -s STATE_DIR -a ACTION -n NAME
#
# OPTIONS
#
#    -s STATE_DIR    State directory
#    -a ACTION       Action to perform, must be one of "create" or "delete"
#    -n NAME         Cluster name to perform action on
#
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
while getopts "s:a:n:" opt; do
    case "$opt" in
	s) state_dir="$OPTARG" ;;
	a) action="$OPTARG" ;;
	n) name="$OPTARG" ;;
	?) die "Unknown option"
    esac
done

if [ -z "$state_dir" ]; then
    die "-s STATE_DIR option required"
fi

if [ ! -d "$state_dir" ]; then
    die "-s STATE_DIR directory does not exist"
fi

if [ -z "$action" ]; then
    die "-a ACTION option required"
fi

if [[ ! "$action" =~ ^create|delete$ ]]; then
    die "-a ACTION must be \"create\" or \"delete\""
fi

if [ -z "$name" ]; then
    die "-n NAME option required"
fi

# Ensure we have all the bins we need
for prog in openshift-install; do
    if ! which "$prog" &> /dev/null; then
	die "$prog must be installed"
    fi
done

# Perform action
cd "$state_dir"
cluster_d="$state_dir/$name"

case "$action" in
    create)
	bold "Creating $name"
	
	# Create install configuration
	if ! mkdir -p "$cluster_d"; then
	    die "Failed to make cluster directory"
	fi
	
	config_f="$cluster_d/install-config.yaml"
	
	if ! "$prog_dir/openshift-install-create-config.yaml.sh" "$name" > "$config_f"; then
	    die "Failed to create openshift-install configuration file"
	fi

	echo "Created openshift-install configuration"
	
	if ! openshift-install create cluster --dir "$cluster_d"; then
	    die "Failed to create cluster $name"
	fi

	echo "Created $name"
	;;
    delete)
	bold "Deleting $name"

	if [ ! -d "$cluster_d" ]; then
	    die "Cluster directory does not exist"
	fi

	if ! openshift-install destroy cluster --dir "$cluster_d"; then
	    die "Failed to delete cluster $name"
	fi

	echo "Deleted $name"
	;;
esac

