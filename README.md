# Auto Cluster
Applies the operator pattern to creating and configuring OpenShift clusters 
on AWS.

# Table Of Contents
- [Overview](#overview)
- [User Guide](#user-guide)
- [Develop](#develop)

# Overview
Manages OpenShift cluster creation and configuration on AWS.

Declare a desired state in a configuration file.  
Run Auto Cluster and let is reconcile the state.

# User Guide
## Cluster Archetypes
The Auto Cluster tool creates clusters based on **cluster archetypes** which 
you define.

A **cluster archetype** contains general cluster level configuration. Such as 
the cluster name or a Helm chart to install on the cluster.

Define cluster archetypes in a YAML file:

```
archetypes:
  - namePrefix: myprefix
	install:
      helmChart: https://github.com/kscout/monochart.git
```

## Configuration
The tool accepts YAML configuration files in the processes working directory or 
in `/etc/auto-cluster`. Defined by the [`config.Config` struct](https://godoc.org/github.com/kscout/auto-cluster/config#Config).

The tool must also be given AWS credentials. Do this via the normal methods 
(ie., `AWS_` environment variables and / or `~/.aws/credentials`file).

# Develop
Auto Cluster tool development instructions.

The `main.go` file is the entry point.

Run the program locally in a container:

```
make container
make run
```
