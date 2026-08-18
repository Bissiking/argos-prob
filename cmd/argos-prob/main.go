// cmd/argos-prob/main.go
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Bissiking/argos-prob/internal/capabilities"
	"github.com/Bissiking/argos-prob/internal/config"
	"github.com/Bissiking/argos-prob/internal/doctor"
	"github.com/Bissiking/argos-prob/internal/host"
	"github.com/Bissiking/argos-prob/internal/transport"
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
	case "run":
		err = runRun()
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
  argos-prob init      Associer cet hôte au master Argos (URL + jeton d'invitation)
  argos-prob run       Daemon: attend l'approbation puis pousse les métriques
  argos-prob status    Afficher l'inventaire local au format JSON
  argos-prob doctor    Diagnostic local
  argos-prob version   Version

Arguments d'init:
  --url URL    URL du master (ou interactive)
  --token AR-… Jeton d'invitation émis par le master (ou interactive)

Le master accepte ou refuse la demande d'association ; après approbation,
'argos-prob run' pousse les métriques à l'intervalle consigné par le master.`)
}

func runInit() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, path, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	cfg.Mode = config.ModePassive

	endpoint, token := initFlags()
	if endpoint == "" {
		endpoint = strings.TrimSpace(prompt("URL du master"))
	}
	if token == "" {
		token = strings.TrimSpace(prompt("Jeton d'invitation (AR-…)"))
	}
	if endpoint == "" || token == "" {
		return fmt.Errorf("une URL de master et un jeton sont requis (flags --url/--token ou saisie interactive)")
	}
	cfg.Endpoint = strings.TrimRight(endpoint, "/")
	cfg.Token = token

	if err := config.Save(path, cfg); err != nil {
		return err
	}
	fmt.Printf("Configuration écrite: %s\n", path)

	hostname := cfg.Name
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	status, err := transport.Associate(ctx, cfg, hostname)
	if err != nil {
		return fmt.Errorf("le master est injoignable ou a refusé le jeton (%v)", err)
	}
	switch status {
	case "approved":
		fmt.Println("Association déjà approuvée : vous pouvez lancer `argos-prob run`.")
	case "rejected":
		return fmt.Errorf("l'association a été refusée par le master (générez une nouvelle invitation)")
	default:
		fmt.Println("Demande d'association envoyée : le master doit l'accepter, puis lancez `argos-prob run`.")
	}
	return nil
}

func initFlags() (string, string) {
	var endpoint, token string
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--url", "--endpoint":
			if i+1 < len(args) {
				endpoint = args[i+1]
				i++
			}
		case "--token":
			if i+1 < len(args) {
				token = args[i+1]
				i++
			}
		}
	}
	return endpoint, token
}

func prompt(label string) string {
	fmt.Print(label + " : ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

func runRun() error {
	cfg, _, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch cfg.TransportMode() {
	case config.ModeActive:
		return transport.Serve(ctx, cfg)
	default:
		return transport.PushLoop(ctx, cfg)
	}
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
