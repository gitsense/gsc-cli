package rules

import "github.com/spf13/cobra"

func templateCmd() *cobra.Command {
	var ruleType string

	cmd := &cobra.Command{
		Use:   "template",
		Short: "Print a rule template",
		Long: `Print a rule-shaped JSON template.

This is the symmetric form of:
  gsc rules new --template

Use --type executable to print an executable rule template.`,
		Example: `  # Print a declarative rule template
  gsc rules template

  # Print an executable rule template
  gsc rules template --type executable`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printRuleTemplate(cmd.OutOrStdout(), ruleType)
		},
	}

	cmd.Flags().StringVar(&ruleType, "type", "", "Rule type: declarative (default), executable")
	return cmd
}
