# Auto Cluster
Automatically creates a new cluster when the current one is reaching the end of
its 48 hour lifespan.

# Table Of Contents
- [Overview](#overview)
- [Run](#run)

# Overview
If a new cluster is needed program will:

- Provision new cluster with `openshift-install` tool
- ~Deploy resources in its namespace to new cluster~ (Out of scope for now,
  Maybe one day)
- Point DNS to new cluster
- Post new credentials to Slack

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

If you have a `~/.aws/credentials` file and the credentials you wish to use are
the default profile you do not have to do anything. If you do not want to use
the default profile set `AWS_PROFILE`.

If you do not have a `~/.aws/credentials` file set `AWS_ACCESS_KEY_ID`
and `AWS_SECRET_ACCESS_KEY`.

## Configuration File
A configuration file is required. Modify the following configuration file with
your information. Save as a `.toml` file and place in the repository root.

```
[Cluster]
NamePrefix = "NAME PREFIX"
OldestAge = 42 # hours, default

[Cloudflare]
Email = "CLOUDFLARE EMAIL"
APIKey = "GLOBAL API KEY"
ZoneID = "ZONEID"

[OpenShiftInstall]
StateStorePath = "PATH TO A DIRECTORY WHICH SCRIPT CAN WRITE TO"

[Slack]
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
