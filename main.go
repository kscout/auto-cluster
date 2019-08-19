package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Noah-Huppert/goconf"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	ec2Svc "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/cloudflare/cloudflare-go"
	"gopkg.in/go-playground/validator.v9"
)

// loggerChild makes a log.Logger from an existing log.Logger
func loggerChild(from *log.Logger, prefix string) *log.Logger {
	return log.New(from.Writer(), fmt.Sprintf("%s.%s", from.Prefix(), prefix),
		from.Flags())
}

// Config holds configuration
type Config struct {
	// Cluster configuration
	Cluster struct {
		// NamePrefix is the prefix to add to cluster names.
		// This value must only contain alphanumeric characters and dashes.
		NamePrefix string `validate:"required,alphadash"`

		// OldestAge a cluster can be before being deleted, in hours
		OldestAge float64 `validate:"min=0,max=48" default:"42"`

		// Namespace to migrate
		Namespace string `validate:"required"`
	} `validate:"required"`

	// Cloudflare configuration
	Cloudflare struct {
		// Email address of account
		Email string `validate:"required"`

		// APIKey
		APIKey string `validate:"required"`

		// ZoneID is the ID of the zone on Cloudflare to configure
		ZoneID string `validate:"required"`
	} `validate:"required"`

	// OpenShiftInstall tool configuration
	OpenShiftInstall struct {
		// StateStorePath is the directory openshift-install state is stored
		StateStorePath string `validate:"required"`
	} `validate:"required"`

	// Slack configuration
	Slack struct {
		// IncomingWebhook is a Slack API incoming webhook to a channel where the new cluster's credentials will be placed
		IncomingWebhook string `validate:"required"`
	} `validate:"required"`

	// Helm configures a Helm chart to be installed on new clusters
	Helm struct {
		// Chart is the URI of a Git repository which holds the chart to install in its root directory
		Chart string
	}
}

// Flags provided by command line invocation
type Flags struct {
	// Once indicates the control loop should only be run once and then the program should exit
	Once bool

	// DryRun makes program not perform any actions instead will output what
	// would happen to stdout
	DryRun bool

	// NoDNS indicates the control loop should not modify DNS records
	NoDNS bool
}

// Cluster is the state of a cluster
type Cluster struct {
	// Name of cluster
	Name string

	// Age of cluster
	Age time.Duration

	// DNSPointed indicates if the Cloudflare DNS zone is pointing to the AWS Route53 zone
	// for the cluster
	DNSPointed bool
}

// String representation of Cluster
func (c Cluster) String() string {
	return fmt.Sprintf("Name=%s, Age=%s, DNSPointed=%t",
		c.Name, c.Age.String(), c.DNSPointed)
}

// EC2Instance holds relevant EC2 instance information
type EC2Instance struct {
	// Name tag
	Name string

	// CreatedOn
	CreatedOn time.Time
}

// String representation of EC2Instance
func (i EC2Instance) String() string {
	return fmt.Sprintf("Name=%s, CreatedOn=%s",
		i.Name, i.CreatedOn.String())
}

// CFDNSRecord holds relevant Cloudflare CNAME DNS record information
type CFDNSRecord struct {
	// ClusterName to which the record points
	ClusterName string

	// Record is the raw DNS record structure
	Record cloudflare.DNSRecord
}

// String representation of CFDNSRecord
func (r CFDNSRecord) String() string {
	return fmt.Sprintf("ClusterName=%s, Record.Name=%s, Record.ID=%s, Record.Content=%s",
		r.ClusterName, r.Record.Name, r.Record.ID, r.Record.Content)
}

// OSInstallPlan is a plan of actions for the openshift-install tool
type OSInstallPlan struct {
	// Create clusters. The Cluster.Name field is the only value used.
	Create []Cluster

	// Delete clusters. The Cluster.Name field is the only value used.
	Delete []Cluster
}

// String representation of OSInstallPlan
func (p OSInstallPlan) String() string {
	createNames := []string{}
	for _, cluster := range p.Create {
		createNames = append(createNames, cluster.Name)
	}

	deleteNames := []string{}
	for _, cluster := range p.Delete {
		deleteNames = append(deleteNames, cluster.Name)
	}

	return fmt.Sprintf("Create=[%s], Delete=[%s]",
		strings.Join(createNames, ","),
		strings.Join(deleteNames, ","))
}

// CFDNSPlan is a plan of actions for Cloudflare DNS
type CFDNSPlan struct {
	// Set DNS records. The CFDNSRecord.Record.Content and
	// CFDNSRecord.Record.ID fields are the only values used.
	Set []CFDNSRecord
}

// String representation of CFDNSPlan
func (p CFDNSPlan) String() string {
	setStrs := []string{}

	for _, record := range p.Set {
		setStrs = append(setStrs, record.String())
	}

	return fmt.Sprintf("Set=[%s]", strings.Join(setStrs, ", "))
}

// HelmInstallPlan is a plan to install a Helm chart on a Kubernetes cluster
type HelmInstallPlan struct {
	// ChartGitURI is the location of a Git repo holding the Helm chart to install
	ChartGitURI string

	// Cluster to install Helm chart on, only .Name field is used
	Cluster Cluster

	// Namespace to install Helm chart in
	Namespace string
}

// String returns a human readable representation of a Helm install plan
func (p HelmInstallPlan) String() string {
	return fmt.Sprintf("chart=%s, cluster=%s, namespace=%s", p.ChartGitURI, p.Cluster.Name,
		p.Namespace)
}

// runCmd runs a command as a subprocess, handles printing out stdout and stderr
func runCmd(stdoutLogger, stderrLogger *log.Logger, cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %s", err.Error())
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			stdoutLogger.Print(scanner.Text())
		}
	}()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %s", err.Error())
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			stdoutLogger.Print(scanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %s", err.Error())
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to wait for command to complete: %s", err.Error())
	}

	return nil
}

func main() {
	// {{{1 Initial setup
	// {{{2 Logger
	logger := log.New(os.Stdout, "auto-cluster ", log.Ldate|log.Ltime)

	// {{{2 Graceful exit
	ctx, cancelCtx := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	go func() {
		<-sigs
		cancelCtx()
		logger.Print("received interrupt signal, will exit gracefully at the " +
			"end of the next controll loop execution")
	}()

	// {{{2 Configuration
	cfgLdr := goconf.NewDefaultLoader()
	cfgLdr.AddConfigPath("/etc/auto-cluster/*.toml")
	cfgLdr.AddConfigPath("./*.toml")

	// {{{3 Custom configuration validators
	alphaDashExp := regexp.MustCompile("^[a-zA-Z-]*$")
	cfgLdr.GetValidate().RegisterValidation("alphadash", func(fl validator.FieldLevel) bool {
		return alphaDashExp.MatchString(fl.Field().String())
	})

	// {{{3 Load
	cfg := Config{}
	if err := cfgLdr.Load(&cfg); err != nil {
		logger.Fatalf("failed to load configuration: %s", err.Error())
	}

	// {{{2 Command line arguments
	flags := Flags{}
	flag.BoolVar(&flags.Once, "once", false, "run control loop once and exit")
	flag.BoolVar(&flags.DryRun, "dry-run", false, "do not perform actions")
	flag.BoolVar(&flags.NoDNS, "no-dns", false, "do not modify DNS")
	flag.Parse()

	// {{{2 Find auxiliary scripts
	cwd, err := os.Getwd()
	if err != nil {
		logger.Fatalf("failed to get working directory: %s", err.Error())
	}

	// {{{3 run-openshift-install.sh script
	runOpenShiftInstallScript := filepath.Join(cwd,
		"scripts/run-openshift-install.sh")
	if _, err := os.Stat(runOpenShiftInstallScript); err != nil {
		logger.Fatalf("failed to stat scripts/run-openshift-install.sh: %s",
			err.Error())
	}

	// {{{3 install-helm-chart.sh
	installHelmChartScript := filepath.Join(cwd, "scripts/install-helm-chart.sh")
	if _, err := os.Stat(installHelmChartScript); err != nil {
		logger.Fatalf("failed to stat scripts/migrate-cluster.sh: %s",
			err.Error())
	}

	// {{{1 API setup
	// {{{2 AWS
	awsSess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	})
	if err != nil {
		logger.Fatalf("failed to create AWS session: %s", err.Error())
	}

	ec2 := ec2Svc.New(awsSess)

	// {{{2 Cloudflare
	cf, err := cloudflare.New(cfg.Cloudflare.APIKey, cfg.Cloudflare.Email)
	if err != nil {
		logger.Fatalf("failed to create a Cloudflare client: %s", err.Error())
	}

	// {{{1 Control loop
	if flags.Once {
		logger.Print("running control loop once")
	}

	ctrlLoopTimer := time.NewTimer(0)

	for {
		select {
		case <-ctx.Done():
			logger.Print("control loop execution finished, exiting")
			return
			break
		case <-ctrlLoopTimer.C:
			// {{{2 Get state
			logger.Print("get state stage")

			// clusters found, keys are cluster names
			clusters := map[string]Cluster{}

			// {{{3 Get DNS entries
			rawRecords, err := cf.DNSRecords(cfg.Cloudflare.ZoneID, cloudflare.DNSRecord{
				Type: "CNAME",
			})
			if err != nil {
				logger.Fatalf("failed to get Cloudflare DNS records: %s",
					err.Error())
			}

			records := []CFDNSRecord{}
		RECORDS_FOR:
			for _, record := range rawRecords {
				if !strings.Contains(record.Content, cfg.Cluster.NamePrefix) {
					continue
				}

				for _, part := range strings.Split(record.Content, ".") {
					if strings.HasPrefix(part, cfg.Cluster.NamePrefix) {
						cfDNSRecord := CFDNSRecord{
							ClusterName: part,
							Record:      record,
						}
						records = append(records, cfDNSRecord)
						logger.Printf("found Cloudflare DNS record: %s", cfDNSRecord.String())

						continue RECORDS_FOR
					}
				}
			}

			// {{{3 Determine which cluster DNS records are currently pointing at
			// recordsCluster is the name of the cluster which records point to.
			// If this value is empty after the following for loop then the records
			// are pointing to multiple clusters
			recordsCluster := ""

			for _, record := range records {
				if recordsCluster == "" {
					recordsCluster = record.ClusterName
				} else if recordsCluster != record.ClusterName {
					recordsCluster = ""
					break
				}
			}

			// {{{3 Get EC2 instances who's names match Config.Cluster.NamePrefix
			ec2NextToken := aws.String("")

			clusterInstances := []EC2Instance{}
			for {
				ec2DescInput := &ec2Svc.DescribeInstancesInput{
					NextToken: ec2NextToken,
				}

				resp, err := ec2.DescribeInstances(ec2DescInput)
				if err != nil {
					logger.Fatalf("failed to describe AWS EC2 instances: %s",
						err.Error())
				}

				for _, reservation := range resp.Reservations {
					// For each instance
				INSTANCES_FOR:
					for _, instance := range reservation.Instances {
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
								if strings.HasPrefix(*tag.Value, cfg.Cluster.NamePrefix) {
									ec2Instance := EC2Instance{
										Name:      *tag.Value,
										CreatedOn: *instance.LaunchTime,
									}
									clusterInstances = append(clusterInstances, ec2Instance)

									logger.Printf("found AWS EC2 instance: %s", ec2Instance.String())
									continue INSTANCES_FOR
								}
							}
						}
					}
				}

				// Paginate if we need to
				ec2NextToken = resp.NextToken
				if ec2NextToken == nil {
					break
				}
			}

			// {{{3 Group matching EC2 instances into clusters
			for _, instance := range clusterInstances {
				// {{{4 Get cluster name from instance name
				parts := strings.Split(instance.Name, "-")

				i := 0
				clusterName := ""

				for !strings.HasPrefix(clusterName, cfg.Cluster.NamePrefix) && i < len(parts) {
					clusterName = strings.Join(parts[:i], "-")
					i += 1
				}

				if !strings.HasPrefix(clusterName, cfg.Cluster.NamePrefix) {
					logger.Fatalf("instance %s was selected as part of cluster but could not extract cluster name",
						instance.Name)
				}

				// {{{4 Create cluster
				if _, ok := clusters[clusterName]; ok {
					continue
				}

				clusters[clusterName] = Cluster{
					Name:       clusterName,
					Age:        time.Since(instance.CreatedOn),
					DNSPointed: clusterName == recordsCluster,
				}
			}

			for _, cluster := range clusters {
				logger.Printf("found cluster: %s", cluster.String())
			}

			// {{{2 Determine what must be done given existing state
			logger.Print("plan stage")

			// {{{3 OpenShift install plan
			osInstallPlan := OSInstallPlan{
				Delete: []Cluster{},
				Create: []Cluster{},
			}

			// youngClusters is a list of clusters which are less than
			// Config.Cluster.OldestAge hours old
			youngClusters := []Cluster{}

			// primaryCluster is the cluster, existing or to be created, which
			// will be used to host the site. This means developers will access this
			// cluster via oc and end users will access this cluster via a domain.
			var primaryCluster *Cluster = nil

			// {{{4 Group clusters as old (older than cfg.Cluster.OldestAge) or young
			for _, cluster := range clusters {
				// Plan to delete old clusters
				if cluster.Age.Hours() > cfg.Cluster.OldestAge {
					osInstallPlan.Delete = append(osInstallPlan.Delete,
						cluster)
				} else {
					youngClusters = append(youngClusters, cluster)
				}
			}

			// {{{4 Figure out what to do with young clusters
			if len(youngClusters) == 0 { // If no young clusters we have to create a new one
				// {{{5 Get next cluster number
				maxClusterNum := int64(0)

				// {{{6 Find highest numeric value in openshift install data store path
				dirs, err := ioutil.ReadDir(cfg.OpenShiftInstall.StateStorePath)
				if err != nil {
					logger.Fatalf("failed to read existing cluster credentials directory")
				}

				for _, dir := range dirs {
					if !dir.IsDir() {
						continue
					}

					if !strings.HasPrefix(dir.Name(), cfg.Cluster.NamePrefix) {
						continue
					}

					numStr := strings.ReplaceAll(dir.Name(),
						cfg.Cluster.NamePrefix, "")
					num, err := strconv.ParseInt(numStr, 10, 64)
					if err != nil {
						logger.Fatalf("failed to parse cluster number for "+
							"previous cluster credentials directory %s: %s",
							dir.Name(), err.Error())
					}

					if num > maxClusterNum {
						maxClusterNum = num
					}
				}

				// {{{6 Add 1 to highest found numeric prefix
				nextClusterNum := maxClusterNum + 1
				nextClusterNumStr := fmt.Sprintf("%d", nextClusterNum)
				if nextClusterNum < 10 {
					nextClusterNumStr = fmt.Sprintf("0%s",
						nextClusterNumStr)
				}

				// {{{5 Plan to create new cluster
				c := Cluster{
					Name: fmt.Sprintf("%s%s", cfg.Cluster.NamePrefix,
						nextClusterNumStr),
				}

				osInstallPlan.Create = []Cluster{c}
				primaryCluster = &c

			} else if len(youngClusters) > 1 { // More than 1 young clusters exist, delete all but the youngest
				// {{{5 Find youngest cluster
				youngestAge := float64(48)
				youngestName := ""

				for _, cluster := range youngClusters {

					if cluster.Age.Hours() < youngestAge {
						youngestAge = cluster.Age.Hours()
						youngestName = cluster.Name
					}
				}

				// {{{5 Plan to delete all but youngest cluster
				for _, cluster := range youngClusters {
					if cluster.Name != youngestName {
						osInstallPlan.Delete = append(osInstallPlan.Delete,
							cluster)
					} else {
						primaryCluster = &cluster
					}
				}
			} else if len(youngClusters) == 1 { // If exactly 1 young cluster, keep
				primaryCluster = &youngClusters[0]
			}

			if primaryCluster == nil {
				logger.Fatal("failed to resolve primary cluster")
			}

			// {{{3 Cloudflare DNS plan
			cfDNSPlan := CFDNSPlan{
				Set: []CFDNSRecord{},
			}

			if !primaryCluster.DNSPointed {
				// Point all records to primary cluster
				for _, record := range records {
					if record.ClusterName == primaryCluster.Name {
						continue
					}

					record.Record.Content = strings.ReplaceAll(record.Record.Content,
						record.ClusterName, primaryCluster.Name)
					cfDNSPlan.Set = append(cfDNSPlan.Set, record)
				}
			}

			// {{{3 Helm install plan
			var helmPlan *HelmInstallPlan = nil

			// If DNS pointed to a different cluster probably means primary cluster used to be
			// a different.
			if primaryCluster.Name != recordsCluster && len(cfg.Helm.Chart) > 0 {
				// If cluster DNS is pointing to exists, then migrate from
				if _, ok := clusters[recordsCluster]; ok {
					helmPlan = &HelmInstallPlan{
						Cluster: Cluster{
							Name: recordsCluster,
						},
						ChartGitURI: cfg.Helm.Chart,
						Namespace:   cfg.Cluster.Namespace,
					}
				}
			}

			// {{{3 Log plan
			logger.Printf("OpenShift install plan: %s", osInstallPlan)

			logger.Printf("Cloudflare DNS plan: %s", cfDNSPlan)

			if helmPlan == nil {
				logger.Printf("helm plan: none")
			} else {
				logger.Printf("helm plan: %s", *helmPlan)
			}
			logger.Printf("primary cluster=%s", *primaryCluster)

			// {{{3 Execute plans
			logger.Print("execute stage")
			// {{{4 OpenShift install create
			logger.Printf("execute OpenShift install create")

			for _, cluster := range osInstallPlan.Create {
				// {{{5 Dry run
				if flags.DryRun {
					logger.Printf("would exec %s -s %s -a create -n %s",
						runOpenShiftInstallScript,
						cfg.OpenShiftInstall.StateStorePath,
						cluster.Name)
					logger.Print("would message Slack with new credentials")
					continue
				}

				// {{{5 Create cluster
				cmd := exec.Command(runOpenShiftInstallScript,
					"-s", cfg.OpenShiftInstall.StateStorePath,
					"-a", "create",
					"-n", cluster.Name)
				err := runCmd(loggerChild(logger, "openshift-install.create.stdout"),
					loggerChild(logger, "openshift-install.create.stderr"), cmd)
				if err != nil {
					logger.Fatalf("failed to create cluster %s: %s",
						cluster.Name, err.Error())
				}

				logger.Printf("created cluster %s", cluster.Name)

				// {{{5 Post new credentials to Slack
				// {{{6 Get kubeadmin user dashboard password
				kubeadminPw, err := ioutil.ReadFile(filepath.Join(
					cfg.OpenShiftInstall.StateStorePath,
					cluster.Name, "auth", "kubeadmin-password"))
				if err != nil {
					logger.Fatalf("failed to open kubeadmin-password file for "+
						"cluster %s: %s", err.Error())
				}

				// {{{6 Encode Slack message as JSON
				buf := bytes.NewBuffer([]byte{})
				encoder := json.NewEncoder(buf)
				msg := map[string]string{
					"text": fmt.Sprintf("*New temporary OpenShift 4.1 cluster*\n"+
						"*URL*: `https://console-openshift-console.apps.%s.devcluster.openshift.com`\n"+
						"*Username*: `kubeadmin`\n"+
						"*Password*: `%s`",
						cluster.Name, string(kubeadminPw)),
				}
				if err := encoder.Encode(msg); err != nil {
					logger.Fatalf("failed to encode Slack message as JSON: %s",
						err.Error())
				}

				// {{{6 Send to Slack
				_, err = http.Post(cfg.Slack.IncomingWebhook, "application/json", buf)
				if err != nil {
					logger.Fatalf("failed to post Slack message for cluster %s: %s",
						cluster.Name, err.Error())
				}
			}

			// {{{4 Helm chart install
			logger.Printf("execute Helm chart install")
			if helmPlan != nil {
				if flags.DryRun {
					logger.Printf("would exec %s -s %s -c %s -n %s %s",
						installHelmChartScript,
						cfg.OpenShiftInstall.StateStorePath,
						helmPlan.Cluster.Name,
						helmPlan.Namespace,
						helmPlan.ChartGitURI)

				} else {
					cmd := exec.Command(installHelmChartScript,
						"-s", cfg.OpenShiftInstall.StateStorePath,
						"-c", helmPlan.Cluster.Name,
						"-n", helmPlan.Namespace,
						helmPlan.ChartGitURI)
					err := runCmd(loggerChild(logger, "helm-install.stdout"),
						loggerChild(logger, "helm-install.stderr"), cmd)
					if err != nil {
						logger.Fatalf("failed to install Helm chart \"%s\" in the \"%s\" namespace on the \"%s\" cluster",
							helmPlan.ChartGitURI, helmPlan.Namespace, helmPlan.Cluster.Name)
					}

					logger.Printf("installed Helm chart \"%s\" in the \"%s\" namespace on the \"%s\" cluster",
						helmPlan.ChartGitURI, helmPlan.Namespace, helmPlan.Cluster.Name)
				}
			}

			// {{{4 CloudflareDNS
			logger.Print("execute Cloudflare DNS set")
			for _, record := range cfDNSPlan.Set {
				if flags.DryRun {
					logger.Printf("would set Cloudflare DNS record %s=%s",
						record.Record.Name, record.Record.Content)
					continue
				}

				err := cf.UpdateDNSRecord(cfg.Cloudflare.ZoneID, record.Record.ID,
					record.Record)
				if err != nil {
					logger.Fatalf("failed to update Cloudflare DNS record %s: %s",
						record.Record.Name, err.Error())
				}

				logger.Printf("updated Cloudflare DNS record.Name=%s to record.Content=%s",
					record.Record.Name, record.Record.Content)
			}

			// {{{4 OpenShift install delete
			logger.Printf("execute OpenShift install delete")
			for _, cluster := range osInstallPlan.Delete {
				// {{{5 Dry run
				if flags.DryRun {
					logger.Printf("would exec %s -s %s -a delete -n %s",
						runOpenShiftInstallScript,
						cfg.OpenShiftInstall.StateStorePath,
						cluster.Name)
					continue
				}

				// {{{5 Delete
				cmd := exec.Command(runOpenShiftInstallScript,
					"-s", cfg.OpenShiftInstall.StateStorePath,
					"-a", "delete",
					"-n", cluster.Name)
				err := runCmd(loggerChild(logger, "openshift-install.delete.stdout"),
					loggerChild(logger, "openshift-install.delete.stderr"), cmd)
				if err != nil {
					logger.Fatalf("failed to delete cluster %s: %s",
						cluster.Name, err.Error())
				}

				logger.Printf("delete cluster %s", cluster.Name)
			}

			// {{{2 Determine when to run next control loop
			if flags.Once {
				logger.Print("ran control loop once, exiting")
				os.Exit(0)
			} else {
				logger.Print("ran control loop, sleeping 15m before next iteration")
				ctrlLoopTimer.Reset(time.Minute * 15)
			}
			break
		}
	}
}
