package controller

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kscout/auto-cluster/cluster"
	"github.com/kscout/auto-cluster/config"

	"github.com/aws/aws-sdk-go/aws/session"
	ec2Svc "github.com/aws/aws-sdk-go/service/ec2"
)

// reconcileLoopWait is the time between reconcile loop iterations
// in Controller.Run().
const reconcileLoopWait = 15 * time.Second

// Controller is in charge of reconciling cluster states to match the
// desired state.
type Controller struct {
	// cfg is auto cluster tool configuration
	cfg config.Config

	// ec2 is an AWS EC2 API client
	ec2 *ec2Svc.EC2

	// dryRun indicates the execute stage should not be run
	dryRun bool
}

// NewController creates and initializes a new Controller
func NewController(cfg config.Config, dryRun bool) (Controller, error) {
	c := Controller{
		cfg:    cfg,
		dryRun: dryRun,
	}

	// Connect to AWS API
	awsSess, err := session.NewSession(nil)
	if err != nil {
		return c, fmt.Errorf("failed to create AWS API client: %s",
			err.Error())
	}

	c.ec2 = ec2Svc.New(awsSess)

	return c, nil
}

// Run reconcile loop until context is canceled. Blocks execution.
func (c Controller) Run(ctx context.Context) error {
	// Run reconcile loop immediately for the first iteration
	timer := time.NewTimer(0)

	for {
		select {
		case <-ctx.Done():
			return nil
			break
		case <-timer.C:
			log.Println("running reconcile loop once")

			err := c.reconcile()
			if err != nil {
				return fmt.Errorf("failed to run reconcile loop: %s",
					err.Error())
			}

			log.Printf("ran reconcile loop once, waiting %s before "+
				"next iteration", reconcileLoopWait)

			timer.Reset(reconcileLoopWait)
			break
		}
	}

	return nil
}

// reconcile runs one iteration of the reconcile loop.
// It attempts to make the current state equal the declared desired state.
//
// This function blocks until all actions have been completed. Some actions,
// like creating clusters, can take up to half an hour.
//
// For each ArchetypeSpec in the config file the following is performed:
//
// * Perform garbage collection on clusters based on spec.Replicas.Lifecycle
//
// * Ensure # clusters == spec.Replicas.Count
func (c Controller) reconcile() error {
	// Reconcile each archetype
	for _, spec := range c.cfg.Archetypes {
		log.Printf("reconciling archetype with name prefix \"%s\"",
			spec.NamePrefix)

		// Get status
		status, err := cluster.NewArchetypeStatus(c.ec2, spec)
		if err != nil {
			return fmt.Errorf("failed to get archetype status for spec=%#v: %s",
				spec, err.Error())
		}

		log.Println("status={")
		for _, cluster := range status.Clusters {
			log.Printf("%s,", cluster)
		}
		log.Println("}")

		// Plan
		plan := NewArchetypePlan(spec, status)

		log.Printf("plan=%s", plan)

		// Execute plan
		if !c.dryRun {
			executor := Executor{
				Cfg:    c.cfg,
				Status: status,
				Plan:   plan,
			}
			err = executor.Execute()
			if err != nil {
				return fmt.Errorf("failed to execute plan: %s",
					err.Error())
			}
		} else {
			log.Println("dry run, not executing...")
		}
	}

	return nil
}
