package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandUsesRunloopBranding(t *testing.T) {
	cmd := NewRootCommand("runloop")
	cmd.SetArgs([]string{"--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	help := out.String()
	for _, unwanted := range []string{"Run " + "En" + "clave", "run" + "en" + "clave", "run-" + "en" + "clave"} {
		if strings.Contains(help, unwanted) {
			t.Fatalf("help output contains old project name %q:\n%s", unwanted, help)
		}
	}
	if !strings.Contains(help, "Runloop local workflow executor") {
		t.Fatalf("help output missing Runloop branding:\n%s", help)
	}
}

func TestDaemonCommandUsesRunloopBranding(t *testing.T) {
	cmd := NewDaemonRootCommand()
	cmd.SetArgs([]string{"--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	help := out.String()
	if strings.Contains(help, "Run "+"En"+"clave") || strings.Contains(help, "run"+"en"+"clave"+"d") {
		t.Fatalf("daemon help contains old project name:\n%s", help)
	}
	if !strings.Contains(help, "runloopd") {
		t.Fatalf("daemon help missing runloopd command name:\n%s", help)
	}
}

func TestWorkflowsCommandIncludesManagementCommands(t *testing.T) {
	cmd := NewRootCommand("runloop")
	cmd.SetArgs([]string{"workflows", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	help := out.String()
	for _, want := range []string{"show", "enable", "disable"} {
		if !strings.Contains(help, want) {
			t.Fatalf("workflows help missing %q:\n%s", want, help)
		}
	}
}
