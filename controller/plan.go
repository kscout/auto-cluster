package controller

import (
	"fmt"
	"sort"
	"time"

	"github.com/kscout/auto-cluster/cluster"
)

// ArchetypePlan describes the intent to take actions to resolve a cluster
// archetype's status.
type ArchetypePlan struct {
	// Spec is the cluster archetype spec
	Spec cluster.ArchetypeSpec

	// DeleteClusters are the clusters to delete
	DeleteClusters []cluster.ClusterStatus

	// CreateClusters is the number of clusters to create
	CreateClusters uint
}

// String returns a string representation of a plan
func (p ArchetypePlan) String() string {
	deleteClusterNames := []string{}

	for _, cluster := range p.DeleteClusters {
		deleteClusterNames = append(deleteClusterNames,
			cluster.Name)
	}

	return fmt.Sprintf("DeleteClusters=[%#v], CreateClusters=%d",
		deleteClusterNames, p.CreateClusters)
}

// NewArchetypePlan creates a new plan which will reconcile the desired state
// with the current status
func NewArchetypePlan(spec cluster.ArchetypeSpec,
	status cluster.ArchetypeStatus) ArchetypePlan {

	plan := ArchetypePlan{
		Spec:           spec,
		DeleteClusters: []cluster.ClusterStatus{},
	}

	// Find primary (youngest) cluster
	// This will be used later on in the planning stage.
	// This will be overriden if any new clusters are being created
	var primCluster *cluster.ClusterStatus

	sort.Slice(status.Clusters, func(i, j int) bool {
		return status.Clusters[i].CreatedOn.
			Before(status.Clusters[j].CreatedOn)
	})
	if len(status.Clusters) > 0 {
		primCluster = &(status.Clusters[0])
	}

	// Garbage collect old clusters
	for _, cluster := range status.Clusters {
		// Check if older than spec.Lifecycle.DeleteAfter
		if time.Since(cluster.CreatedOn) >= spec.Replicas.Lifecycle.DeleteAfterDuration {
			plan.DeleteClusters = append(plan.DeleteClusters, cluster)

			// If this is the primary cluster, unset var
			if cluster.Name == primCluster.Name {
				primCluster = nil
			}
		}
	}

	// Make a new primary if the current primary is too old
	if primCluster != nil {
		if time.Since(primCluster.CreatedOn) >= spec.Replicas.Lifecycle.OldestPrimaryDuration {
			primCluster = nil
			plan.CreateClusters++
		}
	}

	// Create / delete more clusters to match spec.Replicas.Count
	afterPlanCount := uint(len(status.Clusters)) + plan.CreateClusters -
		uint(len(plan.DeleteClusters))
	if afterPlanCount > spec.Replicas.Count { // Delete oldest clusters
		delCount := afterPlanCount - spec.Replicas.Count

		for i := uint(0); i < delCount; i++ {
			plan.DeleteClusters = append(plan.DeleteClusters,
				status.Clusters[len(status.Clusters)-1])
			status.Clusters = status.Clusters[:len(status.Clusters)-1]
		}
	} else if afterPlanCount < spec.Replicas.Count { // Create new clusters
		plan.CreateClusters += spec.Replicas.Count - afterPlanCount
	}

	return plan
}
