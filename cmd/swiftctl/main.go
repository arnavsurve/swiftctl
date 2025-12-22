package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asurve/swiftctl/internal/cli"
)

var version = "dev"

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nShutting down...")
		cancel()

		// Give processes time to clean up
		time.Sleep(500 * time.Millisecond)

		// Force exit on second signal
		<-sigChan
		os.Exit(1)
	}()

	if err := cli.Execute(ctx, version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
