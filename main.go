/*
Manages OpenShift cluster creation and configurtion on AWS.
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/kscout/auto-cluster/config"
	"github.com/kscout/auto-cluster/controller"
)

func handleErr(err error, msg string, data ...interface{}) {
	if err != nil {
		log.Fatalf("%s: %s", fmt.Sprintf(msg, data...), err.Error())
	}
}

func main() {
	// Setup context
	ctx, cancelCtx := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	go func() {
		select {
		case <-ctx.Done():
			return
			break
		case <-sigs:
			log.Println("interrupted, shutting down gracefully...")
			cancelCtx()
			break
		}
	}()

	// Get config
	cfg, err := config.NewConfig()
	handleErr(err, "failed to load configuration")
	log.Printf("loaded configuration=%s", cfg)

	// Command line flags
	var dryRun bool
	flag.BoolVar(&dryRun, "dry-run", false, "do not run "+
		"execute stage")
	flag.Parse()

	// Run controller
	ctrl, err := controller.NewController(cfg, dryRun)
	handleErr(err, "failed to create controller")

	err = ctrl.Run(ctx)
	handleErr(err, "failed to run controller reconcile loop")

	log.Println("completed graceful shutdown")
}
