/*
Manages OpenShift cluster creation and configurtion on AWS.
*/
package main

import (
	"context"
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
	log.Printf("loaded configuration=%#v", cfg)

	// Run controller
	ctrl, err := controller.NewController(cfg)
	handleErr(err, "failed to create controller")

	err = ctrl.Run(ctx)
	handleErr(err, "failed to run controller reconcile loop")

	log.Println("completed graceful shutdown")
}
