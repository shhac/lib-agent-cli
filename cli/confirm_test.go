package cli

import (
	"testing"

	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

func TestRequireConfirm(t *testing.T) {
	if err := RequireConfirm(true, "x"); err != nil {
		t.Errorf("yes=true should pass, got %v", err)
	}
	err := RequireConfirm(false, "would delete the widget")
	if err == nil {
		t.Fatal("yes=false should error")
	}
	var oe *output.Error
	if !output.As(err, &oe) || oe.FixableBy != output.FixableByHuman {
		t.Errorf("want fixable_by:human, got %v", err)
	}
}

func TestAddConfirmFlag(t *testing.T) {
	var yes bool
	cmd := &cobra.Command{Use: "x"}
	AddConfirmFlag(cmd, &yes)
	if cmd.Flags().Lookup("yes") == nil {
		t.Error("--yes flag not registered")
	}
}
