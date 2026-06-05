/**
 * Component: Auth Delete Command
 * Block-UUID: 8c7d6e5f-4a3b-4c2d-9e8f-1a2b3c4d5e6f
 * Parent-UUID: 113a4833-0abd-48cf-b85a-9abcafc51a4f
 * Version: 1.0.2
 * Description: Implements the 'gsc app auth delete' command to revoke and remove an auth code.
 * Language: Go
 * Created-at: 2026-05-20T17:01:30.212Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2)
 */


package auth

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	delFormat string
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete <code>",
	Short: "Delete an auth code",
	Long: `Immediately revokes and deletes an auth code.
This is useful for manual cleanup or invalidating a code after use.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		code := args[0]

		// 1. Delete Code
		err := DeleteCode(code)
		
		// 2. Output
		if delFormat == "json" {
			// JSON Output
			output := struct {
				Success bool   `json:"success"`
				Code    string `json:"code"`
				Message string `json:"message,omitempty"`
				Error   string `json:"error,omitempty"`
			}{
				Success: err == nil,
				Code:    code,
			}

			if err != nil {
				if os.IsNotExist(err) {
					output.Error = "auth code not found"
				} else {
					output.Error = err.Error()
				}
			} else {
				output.Message = "auth code deleted"
			}

			jsonBytes, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}
			fmt.Println(string(jsonBytes))
		} else {
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("❌ Error: Auth code %s not found.\n", code)
				} else {
					fmt.Printf("❌ Error: Failed to delete auth code %s: %v\n", code, err)
				}
				return err
			}
			fmt.Printf("✅ Auth code %s deleted successfully.\n", code)
		}

		// Return error if deletion failed, regardless of format
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	deleteCmd.Flags().StringVar(&delFormat, "format", "human", "Output format (human|json)")
	deleteCmd.SilenceUsage = true

	AuthCmd.AddCommand(deleteCmd)
}
