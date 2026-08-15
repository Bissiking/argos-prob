// cmd/argos-prob/main.go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Bissiking/argos-prob/internal/capabilities"
	"github.com/Bissiking/argos-prob/internal/config"
	"github.com/Bissiking/argos-prob/internal/doctor"
	"github.com/Bissiking/argos-prob/internal/host"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = runInit()
	case "status":
		err = runStatus()
	case "doctor":
		err = runDoctor()
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "argos-prob: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`Argos Prob - cross-platform Argos host agent

Usage:
  argos-prob init      Create the local configuration
  argos-prob status    Print local host inventory as JSON
  argos-prob doctor    Run local diagnostics
  argos-prob version   Print version

The remote Argos API is optional in this development version.`)
}

func runInit() error {
	path, created, err := config.Ensure()
	if err != nil {
		return err
	}
	if created {
		fmt.Printf("Configuration created: %s\n", path)
	} else {
		fmt.Printf("Configuration already exists: %s\n", path)
	}
	return nil
}

func runStatus() error {
	cfg, _, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	snapshot, err := host.Collect(cfg.AgentID, capabilities.Detect())
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(snapshot)
}

func runDoctor() error {
	report := doctor.Run()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
