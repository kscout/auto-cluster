# Auto Cluster
Automatically creates a new cluster when the current one is reaching the end of
its 48 hour lifespan.

# Table Of Contents
- [Overview](#overview)
- [Run](#run)
- [Access Clusters](#access-clusters)
- [Container](#container)

# Overview
Ensures no clusters which are getting too old. If any clusters are in danger 
of being deleted the following steps are taken:

- Provision new cluster with the 
  [OpenShift installer tool](https://github.com/openshift/installer)
- Post new cluster credentials to the Slack
- Install Helm chart on new cluster
- Point DNS to new cluster

This tool is tailored for the use case of the KScout team. As such, several 
assumptions are made:

- DNS hosted on Cloudflare
- All applications deployed in the same namespace

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

[Helm]
# Git URI of repository holding Helm chart to install on new clusters
Chart = "CHART GIT URI"
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
To run every 15 minutes:

```
go run .
```

## No DNS
To run the tool and ensure that no DNS changes will be made:

```
go run . -no-dns
```

# Access Clusters
The `auth-cluster-auth` script helps provide access to temporary clusters 
created by the auto cluster tool.

First sync credentials down from the auto cluster instance:

```
./auto-cluster-auth [-n NS,-e ENV] sync
```

Then list available clusters:

```
./auto-cluster-auth [-n NS,-e ENV] ls
```

Finally get the copy the output of the following command and run it in 
your terminal:

```
./auto-cluster-auth [-n NS,-e ENV] env [CLUSTER_NAME]
```

To open the cluster's dashboard run:

```
./auto-cluster-auth [-n NS,-e ENV] browse [CLUSTER_NAME]
```

# Container
The `quay.io/kscout/auto-cluster:latest` Docker image is available for use:

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
make container
```
