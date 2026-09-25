package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Exercise the release linker, not just the default value in the source. A Go
// constant silently ignored -X and caused packages to report an older version.
func TestReleaseVersionIsEmbedded(t *testing.T) {
	name := "argos-prob"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	want := "9.8.7-build-test"
	build := exec.Command("go", "build", "-ldflags", "-X github.com/Bissiking/argos-prob/internal/version.Version="+want, "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	output, err := exec.Command(binary, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != want {
		t.Fatalf("release version = %q, want %q", got, want)
	}
}
