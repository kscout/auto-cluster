package cluster

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	ec2Svc "github.com/aws/aws-sdk-go/service/ec2"
)

// ArchetypeSpec defines the parameters of an OpenShift cluster
type ArchetypeSpec struct {
	// NamePrefix is a prefix to place before a cluster's name.
	//
	// Cluster's will be given unique names based on this prefix. Clusters
	// without this prefix will be ignored by the tool.
	NamePrefix string `mapstructure:"namePrefix" validate:"required"`

	// Replicas configures the creation of multiple clusters.
	//
	// If multiple clusters are created the newest cluster is the "primary"
	// cluster. Traffic will be proxied to this cluster. Other cluster replicas
	// will be kept as backups in case the primary fails.
	Replicas struct {
		// Count is the number of replica clusters to create
		Count uint `mapstructure:"count" default:"2" validate:"required"`

		// Lifecycle configures the cluster ages at certain operations
		// will be performed
		Lifecycle struct {
			// DeleteAfter is the oldest a cluster can be before it will be
			// forcefully deleted
			DeleteAfter time.Duration `mapstructure:"deleteAfter" default:"42h" validate:"required"`

			// OldestPrimary is the oldest a cluster can be and still be used
			// as a primary cluster
			OldestPrimary time.Duration `mapstructure:"oldestPrimary" default:"12h" validate:"required"`
		}
	} `mapstructure:"replicas" validate:"required"`

	// Install configures 1 time setup performed when a cluster is
	// first created
	Install struct {
		// HelmChart is a Git URI pointing to a Helm Chart GitHub repository
		// which will be installed on the cluster.
		HelmChart string `mapstructure:"helmChart"`
	} `mapstructure:"install"`
}

// ArchetypeStatus is the current state of clusters which match an ArchetypeSpec
type ArchetypeStatus struct {
	// Clusters which match archetype spec
	Clusters []ClusterStatus
}

// NewArchetypeStatus returns an ArchetypeStatus for a ArchetypeSpec
func NewArchetypeStatus(ec2 *ec2Svc.EC2, spec ArchetypeSpec) (ArchetypeStatus, error) {
	status := ArchetypeStatus{}

	firstRun := true
	nextTok := aws.String("")
	instances := []EC2Instance{}

	// Get instances matching archetype
	for firstRun || nextTok != nil {
		if firstRun {
			firstRun = false
		}

		resp, err := ec2.DescribeInstances(&ec2Svc.DescribeInstancesInput{
			NextToken: nextTok,
		})
		if err != nil {
			return status, fmt.Errorf("failed to get AWS EC2 instances: %s",
				err.Error())
		}

		for _, resv := range resp.Reservations {
			for _, instance := range resv.Instances {
				// Ensure is running
				// See state code documentation: https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#InstanceState
				// state code 16 is running, anything past running
				// we want to ignore
				if *instance.State.Code > int64(16) {
					continue
				}

				// For each tag
				for _, tag := range instance.Tags {
					// If name tag
					if *tag.Key == "Name" {
						// If name matches cluster prefix
						if strings.HasPrefix(*tag.Value, spec.NamePrefix) {
							instances = append(instances, EC2Instance{
								Name:      *tag.Value,
								CreatedOn: *instance.LaunchTime,
							})
							break
						}
					}
				}
			}
		}

		nextTok = resp.NextToken
	}

	// Group instances into clusters
	// TODO: Group instances by clusters
	// clusters keys are ClusterStatus.Name values
	clusters := map[string]ClusterStatus{}

	for _, instance := range instances {
		// Extract cluster name from instance name
		// Instances will have names like: "xyz25-9kjcx-master-2"
		// Where "xyz" is the prefix. We want to extract "xyz25" as the
		// cluster name.
		parts := strings.Split(instance.Name, "-")
		clusterName := ""

		for i := 0; !strings.HasPrefix(clusterName, spec.NamePrefix) &&
			i < len(parts); i++ {
			clusterName = strings.Join(parts[:i], "-")
		}

		// Save in clusters map
		if clusterStatus, ok := clusters[clusterName]; ok {
			clusterStatus.Instances = append(clusterStatus.Instances,
				instance)
			clusters[clusterName] = clusterStatus
		} else {
			clusters[clusterName] = ClusterStatus{
				Name:      clusterName,
				CreatedOn: instance.CreatedOn,
				Instances: []EC2Instance{instance},
			}
		}
	}

	// Create ArchetypeStatus to return
	for _, clusterStatus := range clusters {
		status.Clusters = append(status.Clusters, clusterStatus)
	}

	return status, nil
}
