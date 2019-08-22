package controller

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/kscout/auto-cluster/cluster"
	"github.com/kscout/auto-cluster/config"

	"github.com/thanhpk/randstr"
)

// clusterCfgData is the data given to the clusterCfgTmpl
type clusterCfgData struct {
	// ClusterName is the name of the cluster to create
	ClusterName string

	// PullSecret is a Red Hat container registry authentication
	// token used by the openshift-install tool to pull
	// OpenShift container images.
	PullSecret string
}

// clusterCfgTmplStr is the Go template used for the openshift-install
// cluster configuration file
const clusterCfgTmplStr = `
apiVersion: v1
baseDomain: devcluster.openshift.com
compute:
- hyperthreading: Enabled
  name: worker
  platform: {}
  replicas: 3
controlPlane:
  hyperthreading: Enabled
  name: master
  platform: {}
  replicas: 3
metadata:
  creationTimestamp: null
  name: {{ .ClusterName }}
networking:
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  machineCIDR: 10.0.0.0/16
  networkType: OpenShiftSDN
  serviceNetwork:
  - 172.30.0.0/16
platform:
  aws:
    region: us-east-1
pullSecret: '{{ .PullSecret }}'
`

// Executor performs the actions described by a plan
type Executor struct {
	// Cfg is the tool configuration
	Cfg config.Config

	// Status of archetype clusters
	Status cluster.ArchetypeStatus

	// Plan to execute
	Plan ArchetypePlan
}

// mkClusterName creates a new cluster name with a random postfix
func mkClusterName(prefix string) string {
	return strings.ToLower(fmt.Sprintf("%s-%s", prefix,
		randstr.String(4)))
}

// Execute plan
func (e Executor) Execute() error {
	// Setup openshift-install cluster config file template
	clusterCfgTmpl := template.New("openshift-install")
	_, err := clusterCfgTmpl.Parse(clusterCfgTmplStr)
	if err != nil {
		return err
	}

	// Create clusters
	for i := uint(0); i < e.Plan.CreateClusters; i++ {
		// Find unique name for cluster
		clusterName := mkClusterName(e.Plan.Spec.NamePrefix)

		firstRun := true
		nameClash := false

		for firstRun || nameClash {
			for _, cluster := range e.Status.Clusters {
				if cluster.Name == clusterName {
					nameClash = true

					clusterName = mkClusterName(
						e.Plan.Spec.NamePrefix)
				}
			}

			firstRun = false
		}

		// Create directory to store cluster information
		clusterCfgDir := filepath.Join(e.Cfg.StateDir,
			clusterName)
		err = os.MkdirAll(clusterCfgDir, 0755)
		if err != nil {
			return err
		}

		// Open openshift-install cluster config file
		clusterCfgF, err := os.OpenFile(
			filepath.Join(clusterCfgDir, "install-config.yaml"),
			os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return err
		}

		// Generate openshift-install cluster config file
		clusterCfg := clusterCfgData{
			ClusterName: clusterName,
			PullSecret:  e.Cfg.PullSecret,
		}
		err = clusterCfgTmpl.Execute(clusterCfgF, clusterCfg)
		if err != nil {
			return fmt.Errorf("failed to create cluster "+
				"configuration for cluster #%d: %s",
				i+1, err.Error())
		}

		// Run openshift-install
		log.Printf("creating cluster with name %s", clusterName)

		cmd := exec.Command("openshift-install", "create",
			"cluster", "--dir", clusterCfgDir)
		err = logRunCmd(cmd)
		if err != nil {
			return fmt.Errorf("failed to create "+
				"cluster #%d: %s", i+1, err.Error())
		}
	}

	// Delete clusters
	for _, cluster := range e.Plan.DeleteClusters {
		log.Printf("deleting cluster with name %s", cluster.Name)

		clusterCfgDir := filepath.Join(e.Cfg.StateDir,
			cluster.Name)

		cmd := exec.Command("openshift-install", "destroy",
			"cluster", "--dir", clusterCfgDir)
		err = logRunCmd(cmd)
		if err != nil {
			return fmt.Errorf("failed to delete cluster %s: %s",
				cluster.Name, err.Error())
		}
	}

	return nil
}

// logRunCmd runs an exec.Command, using the logger
// to output the commands' stdout and stderr
func logRunCmd(cmd *exec.Cmd) error {
	// Setup command output logging
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	go logReader(stdout)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	go logReader(stderr)

	// Run command
	err = cmd.Start()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}

// logReader logs all output from a reader
func logReader(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		log.Println(scanner.Text())
	}

	if scanner.Err() != nil {
		log.Fatalf("failed to read: %s", scanner.Err().Error())
	}
}
