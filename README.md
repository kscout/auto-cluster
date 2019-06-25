# Auto Cluster
Automatically migrates the cluster when it is reaching the end of its 48
hour lifespan.

# Table Of Contents
- [Overview](#overview)
- [Run](#configuration)

# Overview
If a new cluster is needed program will:

- Provision new cluster with `openshift-install` tool
- Deploy resources in its namespace to new cluster
- Point DNS to new cluster

Does this by running a simple control loop:

- Resolve current state
- Determine required actions
- Perform required actions

Assumptions are made about the use case:

- DNS hosted on Cloudflare
  - Holds only CNAME records pointing to the DNS setup by
	`openshift-install` on AWS

# Run
## AWS Credentials
AWS credentials must be provided.

If you have a `~/.aws/credentials` file and the credentials you which to use are
the default profile you do not have to do anything. If you do not want to use
the default profile set `AWS_PROFILE`.

If you do not have a `~/.aws/credentials` file set `AWS_ACCESS_KEY_ID`
and `AWS_SECRET_ACCESS_KEY`.

## One Time Invocation
To run the control loop once:

```
go run . -once
```
