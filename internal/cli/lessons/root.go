/**
 * Component: Lessons CLI Root Command
 * Block-UUID: cad50c17-069b-4398-a641-96e49b22cd76
 * Parent-UUID: c03980ae-2812-400e-b415-2c2e2db9d222
 * Version: 1.2.0
 * Description: Removed discardCmd registration and added buildCmd. Discard command eliminated in favour of --replace on new and direct file deletion.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), Codex GPT-5 (v1.1.0), claude-sonnet-4-6 (v1.2.0)
 */


package lessons

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lessons",
		Short: "Capture confirmed repository lessons for future humans and agents",
		Long: `Capture curated repository knowledge as GitSense lessons.

Knowledge is everything we could remember. Lessons are the parts worth carrying forward.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newCmd())
	cmd.AddCommand(validateCmd())
	cmd.AddCommand(reviewCmd())
	cmd.AddCommand(commitCmd())
	cmd.AddCommand(listCmd())
	cmd.AddCommand(showCmd())
	cmd.AddCommand(deleteCmd())
	cmd.AddCommand(buildCmd())
	return cmd
}
