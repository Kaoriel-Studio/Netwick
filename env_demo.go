//go:build !windows || demo

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaoriel-studio/netwick/internal/app"
	"github.com/kaoriel-studio/netwick/internal/fixes"
	"github.com/kaoriel-studio/netwick/internal/platform/fake"
	"github.com/kaoriel-studio/netwick/internal/services"
	"github.com/kaoriel-studio/netwick/internal/worker"
)

func hardenProcess() {}

func workerMain(args []string) int {
	fmt.Fprintln(os.Stderr, "worker is not available in demo builds")
	return 2
}

func buildEnv(args []string) (*app.Env, []string, func(), error) {
	fs := flag.NewFlagSet("netwick", flag.ContinueOnError)
	demo := fs.String("demo", os.Getenv("NETWICK_DEMO"), "demo scenario")
	_ = fs.Parse(filterFlags(args))
	scenario := *demo
	if scenario == "" {
		scenario = fake.AllOK
	}
	if !fake.Valid(scenario) {
		return nil, nil, nil, errors.New("unknown demo scenario: " + scenario + " (" + strings.Join(fake.Scenarios, ", ") + ")")
	}
	sys := fake.New(scenario)
	sys.ActionDelay = 700 * time.Millisecond
	sys.ProbeDelay = 120 * time.Millisecond
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	configDir := filepath.Join(base, "Netwick-demo")
	dataDir, err := os.MkdirTemp("", "netwick-demo-*")
	if err != nil {
		return nil, nil, nil, err
	}
	env := &app.Env{
		Sys:       sys,
		Act:       sys,
		Runner:    services.LocalRunner{Exec: &worker.Executor{Sys: sys, Act: sys}},
		DataDir:   dataDir,
		ConfigDir: configDir,
		Demo:      true,
		Scenario:  scenario,
		OpenSystem: func(o fixes.Opener) error {
			fmt.Println("demo: would open", o)
			return nil
		},
		Simulate: sys.SimulateGuide,
	}
	return env, fake.Scenarios, func() { _ = os.RemoveAll(dataDir) }, nil
}

func filterFlags(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.HasPrefix(a, "--demo=") || strings.HasPrefix(a, "-demo=") {
			out = append(out, a)
		}
	}
	return out
}

func showFatal(msg string) { fmt.Fprintln(os.Stderr, msg) }

func systemLanguage() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if strings.HasPrefix(strings.ToLower(os.Getenv(k)), "ru") {
			return "ru"
		}
	}
	return "en"
}
