/*
OpenShift cluster.
*/
package cluster

// ClusterStatus defines the current status of an OpenShift cluster
type ClusterStatus struct {
	// Name of cluster
	Name string

	// CreatedOn is the time the cluster was created
	CreatedOn time.Time
}
