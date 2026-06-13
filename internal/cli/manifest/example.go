/**
 * Component: Manifest Example Command
 * Block-UUID: 5304d556-4987-47ce-b9c8-b27cfc952f55
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc manifest example' command, printing manifest format documentation and a working JSON example for agent-assisted manifest creation.
 * Language: Go
 * Created-at: 2026-06-13T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */

package manifest

import (
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/spf13/cobra"
)

var exampleCmd = &cobra.Command{
	Use:   "example",
	Short: "Print the manifest format with a working JSON example",
	Long: `Print complete documentation of the GitSense manifest JSON format,
including a working example. The output is markdown designed for AI
consumption; pipe it to a file or let your agent read it directly.

Primary workflow:
  1. Run: gsc manifest example
  2. Ask your agent to read the output and create a manifest for your repo
  3. Validate: gsc manifest validate <output>.json
  4. Import: gsc manifest import <output>.json`,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprint(os.Stdout, manifest.GenerateExample())
		return nil
	},
}
