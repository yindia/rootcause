package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	"rootcause/pkg/server"
)

func TestMainSuccessFlags(t *testing.T) {
	origRun := runServer
	origExit := exit
	origArgs := os.Args
	origStderr := os.Stderr
	t.Cleanup(func() {
		runServer = origRun
		exit = origExit
		os.Args = origArgs
		os.Stderr = origStderr
	})

	var got server.Options
	runServer = func(ctx context.Context, opts server.Options) error {
		got = opts
		return nil
	}
	exit = func(code int) {
		t.Fatalf("unexpected exit %d", code)
	}
	tmp, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatalf("temp stderr: %v", err)
	}
	os.Stderr = tmp
	os.Args = []string{
		"rootcause",
		"--kubeconfig", "/tmp/kubeconfig",
		"--context", "demo",
		"--toolsets", "k8s,istio",
		"--config", "/tmp/config",
		"--read-only",
		"--disable-destructive",
		"--log-level", "debug",
	}

	main()

	if got.Kubeconfig != "/tmp/kubeconfig" || got.Context != "demo" {
		t.Fatalf("unexpected kubeconfig/context: %#v", got)
	}
	if !reflect.DeepEqual(got.Toolsets, []string{"k8s", "istio"}) {
		t.Fatalf("unexpected toolsets: %#v", got.Toolsets)
	}
	if got.ConfigPath != "/tmp/config" || !got.ReadOnly || !got.DisableDestructive || got.LogLevel != "debug" {
		t.Fatalf("unexpected options: %#v", got)
	}
}

func TestMainErrorExit(t *testing.T) {
	origRun := runServer
	origExit := exit
	origArgs := os.Args
	origStderr := os.Stderr
	t.Cleanup(func() {
		runServer = origRun
		exit = origExit
		os.Args = origArgs
		os.Stderr = origStderr
	})

	runServer = func(ctx context.Context, opts server.Options) error {
		return fmt.Errorf("boom")
	}
	exitCode := 0
	exit = func(code int) {
		exitCode = code
	}
	tmp, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatalf("temp stderr: %v", err)
	}
	os.Stderr = tmp
	os.Args = []string{"rootcause"}

	main()

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
}
