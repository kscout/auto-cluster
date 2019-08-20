package cluster

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	ec2Svc "github.com/aws/aws-sdk-go/service/ec2"
)

// ArchetypeSpec defines the parameters of an OpenShift cluster
type ArchetypeSpec struct {
	// NamePrefix is a prefix to place before a cluster's name.
	//
	// Cluster's will be given unique names based on this prefix. Clusters
	// without this prefix will be ignored by the tool.
	NamePrefix string `mapstructure:"namePrefix"`

	// HelmChart is a Git URI pointing to a Helm Chart GitHub repository which
	// will be installed on the cluster.
	HelmChart string `mapstructure:"helmChart"`
}

// ArchetypeStatus is the current state of clusters which match an ArchetypeSpec
type ArchetypeStatus struct {
	// Clusters which match archetype spec
	Clusters []ClusterStatus
}

// ec2Instance holds information about AWS EC2 instances, used internally to
// resolve clusters
type ec2Instance struct {
	// name of instance
	name string

	// createdOn is the time the instance was created
	createdOn time.Time
}

// GetForSpec returns an ArchetypeStatus for a ArchetypeSpec
func GetForSpec(ec2 *ec2Svc.EC2, spec ArchetypeSpec) (ArchetypeStatus, error) {
	status := ArchetypeStatus{}

	firstRun := true
	nextTok := aws.String("")
	instances := []ec2Instance{}

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
							instances = append(instances, eC2Instance{
								name:      *tag.Value,
								createdOn: *instance.LaunchTime,
							})
							break
						}
					}
				}
			}
		}

		nextTok = resp.NextToken
	}

	return status, nil
}
