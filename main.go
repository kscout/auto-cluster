/*
Manages OpenShift cluster creation and configurtion on AWS.
*/
package main

import (
	"fmt"
	"log"

	"github.com/kscout/auto-cluster/cluster"
	"github.com/kscout/auto-cluster/config"

	"github.com/aws/aws-sdk-go/aws/session"
	ec2Svc "github.com/aws/aws-sdk-go/service/ec2"
)

func handleErr(err error, msg string, data ...interface{}) {
	if err != nil {
		log.Fatalf("%s: %s", fmt.Sprintf(msg, data...), err.Error())
	}
}

func main() {
	// Get config
	cfg, err := config.NewConfig()
	handleErr(err, "failed to load configuration")

	// Connect to AWS API
	awsSess, err := session.NewSession(nil)
	handleErr(err, "failed to create AWS API client")
	ec2 := ec2Svc.New(awsSess)

	// Get archetype statuses
	for _, spec := range cfg.Archetypes {
		status, err := cluster.NewArchetypeStatus(ec2, spec)
		handleErr(err, "failed to get archetype status for spec=%#v", spec)
		log.Printf("status=%#v", status)
	}
}
