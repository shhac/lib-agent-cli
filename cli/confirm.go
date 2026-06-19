package cli

import (
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// RequireConfirm gates a state-changing action: it returns a fixable_by:human
// error describing what would happen when yes is false, and nil when yes is
// true. Pair it with AddConfirmFlag.
//
//	cli.AddConfirmFlag(cmd, &yes)
//	// in RunE:
//	if err := cli.RequireConfirm(yes, "this deletes the widget"); err != nil {
//	    return err
//	}
func RequireConfirm(yes bool, warning string) error {
	if yes {
		return nil
	}
	return output.New(warning, output.FixableByHuman).WithHint("rerun with --yes to proceed")
}

// AddConfirmFlag registers the family's standard --yes flag on cmd, bound to dst.
func AddConfirmFlag(cmd *cobra.Command, dst *bool) {
	cmd.Flags().BoolVar(dst, "yes", false, "Confirm this state-changing request")
}
