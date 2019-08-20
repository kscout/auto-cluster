# Auto Cluster
Applies the operator pattern to creating and configuring OpenShift clusters 
on AWS.

# Table Of Contents
- [Overview](#overview)
- [Use](#use)
- [Develop](#develop)

# Overview
Manages OpenShift cluster creation and configuration on AWS.

Declare a desired state in a configuration file.  
Run Auto Cluster and let is reconcile the state.

# Use
The Auto Cluster tool creates clusters based on **cluster archetypes** which 
you define.

A **cluster archetype** contains general cluster level configuration. Such as 
the cluster name or a Helm chart to install on the cluster.

Define cluster archetypes in the `config.yaml` file:

```
archetypes:
  - namePrefix: mykerbos
    helmChart: https://github.com/kscout/monochart.git
```

# Develop
Auto Cluster tool development instructions.

The `main.go` file is the entry point.

Run the program locally in a container:

```
make container
make run
```
