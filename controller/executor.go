package controller

import (
	"io/ioutil"
	"log"
	"os"
	"text/template"
)

// clusterCfgTmpl is the template used for the openshift-install
// cluster configuration file
const clusterCfgTmpl = `

`

// Executor performs the actions described by a plan
type Executor struct {
	// plan to execute
	Plan Plan

	// clusterCfgF is the openshift-install cluster configuration
	// file generated for the plan.
	clusterCfgF *os.File
}

// Execute plan
func (e Executor) Execute() error {
	defer func() {
		err := e.cleanup()
		if err != nil {
			log.Fatalf("failed to cleanup executor: %s",
				err.Error())
		}
	}()

	// Create clusters
	clusterCfgF, err := ioutil.TempFile(os.TempDir(),
		"auto-cluster-openshift-install-")
	if err != nil {
		return err
	}
	e.clusterCfgF = clusterCfgF

	for i := uint(0); i < e.Plan.CreateClusters; i++ {

	}
}

// cleanup deletes any temporary files created by the executor
func (e Executor) cleanup() error {
	// Cleanup cluster config file
	if e.clusterCfgF != nil {
		err := e.clusterCfgF.Close()
		if err != nil {
			return err
		}
	}
}
