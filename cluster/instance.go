package cluster

import (
	"time"
)

// ec2Instance holds information about AWS EC2 instances, used internally to
// resolve clusters
type EC2Instance struct {
	// Name of instance
	Name string

	// CreatedOn is the time the instance was created
	CreatedOn time.Time
}
