package main

import (
	"time"
	"flag"
	"os"
	"strings"
	"fmt"
	"regexp"
	"strconv"
	"os/exec"
	"path/filepath"
	"io/ioutil"
	"bytes"
	"net/http"
	"encoding/json"
	"bufio"
	
	"github.com/Noah-Huppert/goconf"
	"github.com/Noah-Huppert/golog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	ec2Svc "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/cloudflare/cloudflare-go"
	"gopkg.in/go-playground/validator.v9"
)

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
	}

	// Slack configuration
	Slack struct {
		// IncomingWebhook is a Slack API incoming webhook to a channel where the new cluster's credentials will be placed
		IncomingWebhook string `validate:"required"`
	}
}

// Flags provided by command line invocation
type Flags struct {
	// Once indicates the control loop should only be run once and then the program should exit
	Once bool

	// DryRun does not perform any actions, instead outputs what would happen to stdout
	DryRun bool
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
	return fmt.Sprintf("ClusterName=%s, Record.Name=%s",
		r.ClusterName, r.Record.Name)
}

// OSInstallActionPlan is a plan of actions for the openshift-install tool
type OSInstallActionPlan struct {
	// Create clusters. The Cluster.Name field is the only value used.
	Create []Cluster
	
	// Delete clusters. The Cluster.Name field is the only value used.
	Delete []Cluster
}

// String representation of OSInstallActionPlan
func (p OSInstallActionPlan) String() string {
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

// CFDNSActionPlan is a plan of actions for Cloudflare DNS
type CFDNSActionPlan struct {
	// Set DNS records. The CFDNSRecord.Record.Content and
	// CFDNSRecord.Record.ID fields are the only values used.
	Set []CFDNSRecord
}

// String representation of CFDNSActionPlan
func (p CFDNSActionPlan) String() string {
	out := ""

	for i, record := range p.Set {
		if i > 0 {
			out += "\n"
		}
		out += fmt.Sprintf("set Record.Name=%s to Record.Content=%s",
			record.Record.Name, record.Record.Content)
	}

	return out
}

// MigrateActionPlan is a plan to migrate resources from one OpenShift cluster to another
type MigrateActionPlan struct {
	// From cluster, Cluster.Name field is the only value used.
	From Cluster

	// To cluster, Cluster.Name field is the only value used.
	To Cluster

	// Namespace to migrate
	Namespace string
}

// String representation of MigrateActionPlan
func (p MigrateActionPlan) String() string {
	return fmt.Sprintf("From.Name=%s, To.Name=%s, Namespace=%s",
		p.From.Name, p.To.Name, p.Namespace)
}

// runCmd runs a command as a subprocess, handles printing out stdout and stderr
func runCmd(stdoutLogger, stderrLogger golog.Logger, cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %s", err.Error())
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			stdoutLogger.Debug(scanner.Text())
		}
	}()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %s", err.Error())
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			stdoutLogger.Debug(scanner.Text())
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
	logger := golog.NewStdLogger("auto-cluster")

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
	flag.Parse()

	// {{{2 Find auxiliary scripts
	cwd, err := os.Getwd();
	if err != nil {
		logger.Fatalf("failed to get working directory: %s", err.Error())
	}

	// {{{3 run-openshift-install.sh script
	runOpenShiftInstallScript := filepath.Join(cwd, "run-openshift-install.sh")
	if _, err := os.Stat(runOpenShiftInstallScript); err != nil {
		logger.Fatalf("failed to stat run-openshift-install.sh: %s", err.Error())
	}

	// {{{3 migrate-cluster.sh script
	migrateClusterScript := filepath.Join(cwd, "migrate-cluster.sh")
	if _, err := os.Stat(migrateClusterScript); err != nil {
		logger.Fatalf("failed to stat migrate-cluster.sh: %s", err.Error())
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
		logger.Info("running control loop once")
	}
	
	for {
		// {{{2 Get state
		logger.Debug("get state stage")
		
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
						Record: record,
					}
					records = append(records, cfDNSRecord)
					logger.Debugf("found Cloudflare DNS record: %s", cfDNSRecord.String())
					
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
									Name: *tag.Value,
									CreatedOn: *instance.LaunchTime,
								}
								clusterInstances = append(clusterInstances, ec2Instance)

								logger.Debugf("found AWS EC2 instance: %s", ec2Instance.String())
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
				Name: clusterName,
				Age: time.Since(instance.CreatedOn),
				DNSPointed: clusterName == recordsCluster,
			}
		}

		for _, cluster := range clusters {
			logger.Debugf("found cluster: %s", cluster.String())
		}

		// {{{2 Determine what must be done given existing state
		logger.Debug("plan stage")
		
		// {{{3 OpenShift install plan
		osInstallPlan := OSInstallActionPlan{
			Delete: []Cluster{},
			Create: []Cluster{},
		}

		// youngClusters is a list of clusters which are less than 42 hours old
		youngClusters := []Cluster{}

		// primaryCluster is the cluster, existing or to be created, which
		// will be used
		var primaryCluster *Cluster = nil

		// {{{4 Group clusters as old (older than cfg.Cluster.OldestAge) or young
		for _, cluster := range clusters {
			// Delete old clusters
			if cluster.Age.Hours() > cfg.Cluster.OldestAge {
				osInstallPlan.Delete = append(osInstallPlan.Delete,
					cluster)
			} else {
				youngClusters = append(youngClusters, cluster)
			}
		}

		// {{{4 Figure out what to do with young clusters
		if len(youngClusters) == 0 { // If no young clusters we have to create a new one
			// Get next cluster number
			maxClusterNum := int64(0)

			// Find highest value numeric prefix on cluster names
			for _, cluster := range clusters {
				numStr := strings.ReplaceAll(cluster.Name,
					cfg.Cluster.NamePrefix, "")
				num, err := strconv.ParseInt(numStr, 10, 64)
				if err != nil {
					logger.Fatalf("failed to parse cluster number for "+
						"%s: %s", cluster.Name, err.Error())
				}

				if num > maxClusterNum {
					maxClusterNum = num
				}
			}

			// Add 1 to highest found numeric prefix
			nextClusterNum := maxClusterNum + 1
			nextClusterNumStr := fmt.Sprintf("%d", nextClusterNum)
			if nextClusterNum < 10 {
				nextClusterNumStr = fmt.Sprintf("0%s",
					nextClusterNumStr)
			}

			// Plan to create new cluster
			c := Cluster{
				Name: fmt.Sprintf("%s%s", cfg.Cluster.NamePrefix,
					nextClusterNumStr),
			}
				
			osInstallPlan.Create = []Cluster{ c }
			primaryCluster = &c

		} else if len(youngClusters) > 1 { // More than 1 young clusters exist, delete all but the youngest
			// Find youngest cluster
			youngestAge := float64(48)
			youngestName := ""

			for _, cluster := range youngClusters {

				if cluster.Age.Hours() < youngestAge {
					youngestAge = cluster.Age.Hours()
					youngestName = cluster.Name
				}
			}

			// Plan to delete all but youngest cluster
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
		cfDNSPlan := CFDNSActionPlan{
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

		// {{{3 Cluster migrate plan
		var migratePlan *MigrateActionPlan = nil
		
		if primaryCluster.Name != recordsCluster {
			migratePlan = &MigrateActionPlan{
				From: Cluster{
					Name: recordsCluster,
				},
				To: *primaryCluster,
				Namespace: cfg.Cluster.Namespace,
			}
		}

		// {{{3 Log plan
		logger.Debugf("OpenShift install plan: %s", osInstallPlan)
		logger.Debugf("Cloudflare DNS plan: %s", cfDNSPlan)

		if migratePlan == nil {
			logger.Debugf("migrate plan: none")
		} else {
			logger.Debugf("migrate plan: %s", *migratePlan)
		}
		logger.Debugf("primary cluster=%s", *primaryCluster)

		// {{{3 Execute plans
		logger.Debug("execute stage")
		// {{{4 OpenShift install create
		logger.Debugf("execute OpenShift install create")
		
		for _, cluster := range osInstallPlan.Create {
			// {{{5 Dry run
			if flags.DryRun {
				logger.Debugf("would exec %s -s %s -a create -n %s",
					runOpenShiftInstallScript,
					cfg.OpenShiftInstall.StateStorePath,
					cluster.Name)
				logger.Debug("would message Slack with new credentials")
				continue
			}

			// {{{5 Create cluster
			cmd := exec.Command(runOpenShiftInstallScript,
				"-s", cfg.OpenShiftInstall.StateStorePath,
				"-a", "create",
				"-n", cluster.Name)
			err := runCmd(logger.GetChild("openshift-install.create.stdout"),
				logger.GetChild("openshift-install.create.stderr"), cmd)
			if err != nil {
				logger.Fatalf("failed to create cluster %s: %s",
					cluster.Name, err.Error())
			}

			logger.Debugf("created cluster %s", cluster.Name)

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
				"text": fmt.Sprintf("*New cluster*\n"+
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

		// {{{4 Migrate
		logger.Debugf("execute migrate")
		if migratePlan != nil {
			if flags.DryRun {
				logger.Debugf("would exec %s -s %s -f %s -t %s -n %s",
					migrateClusterScript,
					cfg.OpenShiftInstall.StateStorePath,
					migratePlan.From.Name,
					migratePlan.To.Name,
					cfg.Cluster.Namespace)
				
			} else {
				cmd := exec.Command(migrateClusterScript,
					"-s", cfg.OpenShiftInstall.StateStorePath,
					"-f", migratePlan.From.Name,
					"-t", migratePlan.To.Name,
					"-n", migratePlan.Namespace)
				err := runCmd(logger.GetChild("migrate-cluster.stdout"),
					logger.GetChild("migrate-cluster.stderr"), cmd)
				if err != nil {
					logger.Fatalf("failed to migrate %s namespace from %s cluster to %s cluster",
						migratePlan.Namespace, migratePlan.From.Name,
						migratePlan.To.Name)
				}

				logger.Debugf("migrated %s namespace from %s cluster to %s cluster",
					migratePlan.Namespace, migratePlan.From.Name,
					migratePlan.To.Name)
			}
		}

		// {{{4 CloudflareDNS
		logger.Debug("execute Cloudflare DNS set")
		for _, record := range cfDNSPlan.Set {
			if flags.DryRun {
				logger.Debugf("would set Cloudflare DNS record %s=%s",
					record.Record.Name, record.Record.Content)
				continue
			}

			err := cf.UpdateDNSRecord(cfg.Cloudflare.ZoneID, record.Record.ID,
				record.Record)
			if err != nil {
				logger.Fatalf("failed to update Cloudflare DNS record %s: %s",
					record.Record.Name, err.Error())
			}

			logger.Debugf("updated Cloudflare DNS record.Name=%s to record.Content=%s",
				record.Record.Name, record.Record.Content)
		}

		// {{{4 OpenShift install delete
		logger.Debugf("execute OpenShift install delete")
		for _, cluster := range osInstallPlan.Delete {
			// {{{5 Dry run
			if flags.DryRun {
				logger.Debugf("would exec %s -s %s -a delete -n %s",
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
			err := runCmd(logger.GetChild("openshift-install.delete.stdout"),
				logger.GetChild("openshift-install.delete.stderr"), cmd)
			if err != nil {
				logger.Fatalf("failed to delete cluster %s: %s",
					cluster.Name, err.Error())
			}

			logger.Debugf("delete cluster %s", cluster.Name)
		}

		// {{{2 Determine when to run next control loop
		if flags.Once {
			logger.Info("ran control loop once, exiting")
			os.Exit(0)
		} else {
			logger.Info("ran control loop, sleeping 15m before next iteration")
			time.Sleep(15 * time.Minute)
		}
	}
}
