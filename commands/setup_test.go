// commands/setup_test.go - Tests for the command shell

package commands

import (
	"bytes"
	"strings"
	"testing"
)

// run calls RunCommands in-process (R-DEV-5) and returns its output.
func run(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err = RunCommands(append([]string{"km"}, args...), &out, &errOut)
	return out.String(), errOut.String(), err
}

// R-CLI-4: a missing command is not an error and shows the help on stderr.
func TestMissingCommandShowsHelp(t *testing.T) {
	stdout, stderr, err := run(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "USAGE:") {
		t.Errorf("stderr missing help text:\n%s", stderr)
	}
	if strings.Contains(stderr, "km:") {
		t.Errorf("stderr has an error message:\n%s", stderr)
	}
	if stdout != "" {
		t.Errorf("unexpected stdout:\n%s", stdout)
	}
}

// R-CLI-3: --help works on the program.
func TestHelp(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			stdout, stderr, err := run(t, arg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, want := range []string{
				"km - Generate WireGuard configurations from a common YAML file",
				"USAGE:",
				"--help, -h",
				"--config <config-file>, -c <config-file>", // R-CLI-1
				"[$KWIKMESH_CONFIG]",                       // R-FILE-1
			} {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout missing %q:\n%s", want, stdout)
				}
			}
			if stderr != "" {
				t.Errorf("unexpected stderr: %q", stderr)
			}
		})
	}
}

// R-CLI-4: an unknown command is an error, reported with the km prefix.
func TestUnknownCommand(t *testing.T) {
	stdout, stderr, err := run(t, "bogus")
	if err == nil {
		t.Fatal("expected an error for an unknown command")
	}
	want := `km: unknown command "bogus"` + "\n"
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Errorf("unexpected stdout:\n%s", stdout)
	}
}

// R-CLI-4: an unknown option is an error, reported with the km prefix.
func TestUnknownFlag(t *testing.T) {
	stdout, stderr, err := run(t, "--nope")
	if err == nil {
		t.Fatal("expected an error for an unknown flag")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(stderr, "km: "+err.Error()+"\n") {
		t.Errorf("stderr missing prefixed error:\n%s", stderr)
	}
	if !strings.Contains(stdout, "USAGE:") {
		t.Errorf("stdout missing help text:\n%s", stdout)
	}
}
