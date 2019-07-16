# Auto Cluster
Automatically creates a new cluster when the current one is reaching the end of
its 48 hour lifespan.

# Table Of Contents
- [Overview](#overview)
- [Run](#run)
- [Container](#container)

# Overview
The [kscout.io](https://kscout.io) site is currently hosted on temporary 
OpenShift 4.1 development clusters. These clusters are automatically deleted 
after 48 hours. As a result a new cluster must be created every 2 days. New 
resources must be deployed to this cluster and the Cloudflare DNS zone for
kscout.io must be updated to point to the new cluster. Additionally team members
must be informed of the new cluster's details so they can update their 
local toolchains.

The auto cluster tool automates this entire process.  

It acts similar to a Kubernetes controller, comparing the current and desired 
states and planning how to reconcile differences.

The auto cluster tool will run its control loop every 15 minutes. During each
iteration it will ensure that there are no clusters which are getting too
close to their automated deletion date. If any clusters are in danger of being 
deleted the following steps are taken:

- Provision new cluster with the 
  [OpenShift installer tool](https://github.com/openshift/installer)
- Post new cluster credentials to the Slack
- Migrate resources from the old development cluster to the new
  development cluster
- Point DNS to new cluster
- Delete old development cluster

This tool is tailored for the use case of the KScout team. As such, several 
assumptions are made:

- DNS hosted on Cloudflare
- All applications deployed in the same namespace
- Subset of Kuberntes resources used
  - See the [`migrate_types` variable in 
	`migrate-cluster.sh`](migrate-cluster.sh#L21) for the list of resource types

# Run
## AWS Credentials
AWS credentials must be provided.

If you have a `~/.aws/credentials` file and the credentials you wish to use are
the default profile you do not have to do anything. If you do not want to use
the default profile set `AWS_PROFILE`.

If you do not have a `~/.aws/credentials` file set `AWS_ACCESS_KEY_ID`
and `AWS_SECRET_ACCESS_KEY`.

## Configuration File
A configuration file is required. Modify the following configuration file with
your information. Save as a `.toml` file and place in the repository root.

The auto cluster loads TOML files as configuration from the `/etc/auto-cluster` 
directory and the working directory.

```toml
[Cluster]
# Prefix to add to name when searching for / creating new clusters
NamePrefix = "NAME PREFIX"

# Oldest a cluster can be before it will be replaced
OldestAge = 42 # hours, default

# Namespace to migrate over to new development cluster
Namespace = "YOUR NAMESPACE"

[Cloudflare]
Email = "CLOUDFLARE EMAIL"
APIKey = "GLOBAL API KEY"
ZoneID = "ZONEID"

[OpenShiftInstall]
# Directory where openshift-install will store cluster details
StateStorePath = "PATH TO A DIRECTORY WHICH SCRIPT CAN WRITE TO"

[Slack]
# Slack incoming web hook used to post new cluster credentials
IncomingWebhook = "https://hooks.slack.com/services/SECRET_SLACK_INFO
```

Posting the new cluster credentials to Slack requires that you have an incoming
web hook setup. You can set this up via the Slack API dashboard.

## Dry Run
To see what the tool will do when it executes:

```
go run . -once -dry-run
```

## One Time Invocation
To run the control loop once:

```
go run . -once
```

## Continuous Invocation
*Unstable, wouldn't recommend*  

To run every 15 minutes:

```
go run .
```

## No DNS
To run the tool and ensure that no DNS changes will be made:

```
go run . -no-dns
```

# Container
The `docker.io/kscout/auto-cluster:latest` Docker image is available for use:

```
docker run \
	-it \
	--rm \
	-e AWS_ACCESS_KEY_ID=<aws access key ID> \
	-e AWS_SECRET_ACCESS_KEY=<aws secret access key> \
	-v "$PWD/config.toml:/etc/auto-cluster" \
	kscout/auto-cluster:latest
```

## Container Development
Build and push:

```
make docker
```
