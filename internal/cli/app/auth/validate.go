/**
 * Component: Auth Validate Command
 * Block-UUID: 6bb94633-baf7-4efa-86ac-919127c84508
 * Parent-UUID: 8e0b8228-259a-40cc-b4cd-3a5f164df4eb
 * Version: 1.0.2
 * Description: Implements the 'gsc app auth validate' command to check the validity and permissions of an auth code.
 * Language: Go
 * Created-at: 2026-05-20T16:59:56.841Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 2.5 Flash Lite (v1.0.1), GLM-4.7 (v1.0.2)
 */


package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	valPermission string
	valFormat     string
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate <code>",
	Short: "Validate an auth code",
	Long: `Validates an auth code, checks its expiry, and verifies permissions.
This command is typically used by the backend to authenticate user actions.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		code := args[0]

		// 1. Validate Code
		result, err := ValidateCode(code, valPermission)
		if err != nil {
			// Handle error output based on format
			if valFormat == "json" {
				errorOutput := struct {
					Success bool   `json:"success"`
					Error   string `json:"error"`
				}{
					Success: false,
					Error:   err.Error(),
				}
				jsonBytes, jsonErr := json.MarshalIndent(errorOutput, "", "  ")
				if jsonErr != nil {
					return fmt.Errorf("failed to marshal error output: %w", jsonErr)
				}
				fmt.Println(string(jsonBytes))
				return nil
			}
			return fmt.Errorf("validation error: %w", err)
		}

		// 2. Output
		if valFormat == "json" {
			// JSON Output
			// Ensure ExpiresAt is formatted as ISO 8601 string
			type JSONResult struct {
				Valid              bool     `json:"valid"`
				Code               string   `json:"code"`
				Status             string   `json:"status"`
				ExpiresAt          string   `json:"expires_at,omitempty"`
				Permissions        []string `json:"permissions,omitempty"`
				RequiredPermission string   `json:"required_permission,omitempty"`
				Error              string   `json:"error,omitempty"`
			}

			jsonOutput := JSONResult{
				Valid:              result.Valid,
				Code:               result.Code,
				Status:             result.Status,
				Permissions:        result.Permissions,
				RequiredPermission: result.RequiredPermission,
				Error:              result.Error,
			}

			if !result.ExpiresAt.IsZero() {
				jsonOutput.ExpiresAt = result.ExpiresAt.Format(time.RFC3339)
			}

			jsonBytes, err := json.MarshalIndent(jsonOutput, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}
			fmt.Println(string(jsonBytes))
		} else {
			// Human Output
			if result.Valid {
				fmt.Printf("✅ Auth code %s is valid.\n", result.Code)
				fmt.Printf("Permissions: %v\n", result.Permissions)
				duration := time.Until(result.ExpiresAt)
				fmt.Printf("Expires in:  %s\n", duration.Round(time.Second))
			} else {
				fmt.Printf("❌ Error: Auth code %s ", result.Code)
				switch result.Status {
				case StatusExpired:
					fmt.Printf("expired %s ago.\n", time.Since(result.ExpiresAt).Round(time.Second))
				case StatusPermissionDenied:
					fmt.Printf("lacks the required permission: %s\n", result.RequiredPermission)
					fmt.Printf("Available permissions: %v\n", result.Permissions)
				case StatusNotFound:
					fmt.Println("is invalid or not found.")
				default:
					fmt.Printf("is invalid. Reason: %s\n", result.Error)
				}
			}
		}

		return nil
	},
}

func init() {
	validateCmd.Flags().StringVar(&valPermission, "permission", "", "Check if the code has a specific permission")
	validateCmd.Flags().StringVar(&valFormat, "format", "human", "Output format (human|json)")
	validateCmd.SilenceUsage = true

	AuthCmd.AddCommand(validateCmd)
}
