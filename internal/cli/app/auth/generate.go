/**
 * Component: Auth Generate Command
 * Block-UUID: 7a8f9c2d-4e5b-4a3c-8d9e-1f2a3b4c5d6e
 * Parent-UUID: f4c67736-7dbd-4b55-b57f-ab63fdace71f
 * Version: 1.0.1
 * Description: Implements the 'gsc app auth generate' command to create temporary authentication codes.
 * Language: Go
 * Created-at: 2026-05-20T15:55:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1)
 */


package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var (
	genTTL         int
	genPermissions []string
	genFormat      string
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a temporary auth code",
	Long: `Generates a temporary 6-digit auth code and stores it in data/auth/.
This code can be used to authenticate operations like manifest publishing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Generate Code
		code, err := GenerateRandomCode()
		if err != nil {
			// Handle error output based on format
			if genFormat == "json" {
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
			return fmt.Errorf("failed to generate auth code: %w", err)
		}

		// 2. Save Code
		if err := SaveCode(code, genPermissions, genTTL); err != nil {
			// Handle error output based on format
			if genFormat == "json" {
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
			return fmt.Errorf("failed to save auth code: %w", err)
		}

		// 3. Output
		if genFormat == "json" {
			// JSON Output
			// We need to reload the data to get the exact timestamps
			data, err := LoadCode(code)
			if err != nil {
				// Handle error output based on format
				if genFormat == "json" {
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
				return fmt.Errorf("failed to load saved code for output: %w", err)
			}

			output := struct {
				Code        string    `json:"code"`
				ExpiresAt   string    `json:"expires_at"`
				Permissions []string  `json:"permissions"`
			}{
				Code:        data.Code,
				ExpiresAt:   data.ExpiresAt.Format(time.RFC3339),
				Permissions: data.Permissions,
			}

			jsonBytes, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}
			fmt.Println(string(jsonBytes))
		} else {
			// Human Output
			fmt.Printf("Your auth code is: %s\n", code)
			fmt.Printf("Permissions: %v\n", genPermissions)
			fmt.Printf("Valid for: %d minutes\n", genTTL)
		}

		return nil
	},
}

func init() {
	generateCmd.Flags().IntVar(&genTTL, "ttl", settings.DefaultAuthCodeTTL, "Time-to-live for the code in minutes")
	generateCmd.Flags().StringSliceVar(&genPermissions, "permissions", []string{settings.PermissionManifestPublish}, "Comma-separated list of permissions")
	generateCmd.Flags().StringVar(&genFormat, "format", "human", "Output format (human|json)")
	generateCmd.SilenceUsage = true

	AuthCmd.AddCommand(generateCmd)
}
