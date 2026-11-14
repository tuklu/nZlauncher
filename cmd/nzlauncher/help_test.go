package main

import (
	"slices"
	"strings"
	"testing"
)

// Every command with its own help is in the overview, the manual and each
// completion script, so adding a command without them fails here.
func TestCommandsDocumentedEverywhere(t *testing.T) {
	page, _ := share.ReadFile("share/nzlauncher.1")
	scripts := map[string]string{}
	for shell, file := range completionFiles {
		b, err := share.ReadFile(file)
		if err != nil {
			t.Fatalf("%s completion: %v", shell, err)
		}
		scripts[shell] = string(b)
	}
	for command := range commandHelp {
		if !strings.Contains(mainHelp, "  "+command) {
			t.Errorf("%s is missing from the overview", command)
		}
		if !strings.Contains(string(page), command) {
			t.Errorf("%s is missing from the manual", command)
		}
		for shell, script := range scripts {
			if !strings.Contains(script, command) {
				t.Errorf("%s is missing from the %s completion", command, shell)
			}
		}
	}
}

func TestHelpTextStartsWithItsSynopsis(t *testing.T) {
	for command, text := range commandHelp {
		if !strings.HasPrefix(text, "usage: nzlauncher "+command) {
			t.Errorf("%s help does not open with its usage line", command)
		}
	}
}

func TestOptionEnd(t *testing.T) {
	if !wantsHelp([]string{"connect", "--help"}) || wantsHelp([]string{"--", "--help"}) {
		t.Error("--help after -- is an argument, before it a request for help")
	}
	got := withoutDashDash([]string{"a", "--", "-b", "--"})
	if !slices.Equal(got, []string{"a", "-b", "--"}) {
		t.Errorf("got %v", got)
	}
}
