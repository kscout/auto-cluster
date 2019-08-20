package cluster

import (
	"time"
)

// ClusterStatus defines the current status of an OpenShift cluster
type ClusterStatus struct {
	// Name of cluster
	//
	// This is considered a unique identifier.
	Name string

	// CreatedOn is the time the cluster was created
	CreatedOn time.Time

	// Instances are the AWS EC2 instances which are the nodes of the cluster
	Instances []EC2Instance
}
