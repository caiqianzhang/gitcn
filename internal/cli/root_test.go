package cli

import "testing"

func TestVersionOutput(t *testing.T) {
	if err := Run([]string{"version"}); err != nil {
		t.Fatalf("Run(version) err = %v", err)
	}
}

func TestUnknownCommand(t *testing.T) {
	if err := Run([]string{"nosuchcmd"}); err == nil {
		t.Fatal("expected error for unknown command")
	}
}
