/**
 * Component: Native App CLI Cleanup
 * Block-UUID: dc2c7879-7940-43b5-9634-8bff094ba5cd
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc app native cleanup' command to remove old release archives and free disk space. Supports configurable maximum number of releases to keep.
 * Language: Go
 * Created-at: 2026-05-11T23:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package native

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var (
	cleanupKeep int
	cleanupDry  bool
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove old release archives to free disk space",
	Long: `Removes old versioned release directories from ~/.gitsense/releases/ to free disk space.
By default, keeps the last 3 versions. Use --keep to adjust this number.

This is safe to run at any time - it only removes archived source code,
not the active installation or your data.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		gscHome, err := settings.GetGSCHome(false)
		if err != nil {
			return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
		}

		releasesDir := filepath.Join(gscHome, "releases")

		// Check if releases directory exists
		if _, err := os.Stat(releasesDir); os.IsNotExist(err) {
			fmt.Println("No releases directory found. Nothing to clean up.")
			return nil
		}

		// List all version directories
		entries, err := os.ReadDir(releasesDir)
		if err != nil {
			return fmt.Errorf("failed to read releases directory: %w", err)
		}

		// Filter and collect versions
		var versions []string
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "v") {
				versions = append(versions, entry.Name())
			}
		}

		if len(versions) == 0 {
			fmt.Println("No versioned releases found. Nothing to clean up.")
			return nil
		}

		// Sort versions (newest first)
		// String comparison works for semantic versioning like v0.1.0, v0.2.0, etc.
		sort.Slice(versions, func(i, j int) bool {
			return versions[i] > versions[j]
		})

		// Calculate what to keep and what to remove
		keepCount := cleanupKeep
		if keepCount < 1 {
			keepCount = 1 // Always keep at least the latest version
		}

		var toKeep []string
		var toRemove []string

		if len(versions) <= keepCount {
			toKeep = versions
		} else {
			toKeep = versions[:keepCount]
			toRemove = versions[keepCount:]
		}

		// Show summary
		fmt.Println("\nGitSense Chat Release Cleanup")
		fmt.Println("------------------------------")
		fmt.Printf("  Total releases:  %d\n", len(versions))
		fmt.Printf("  Keeping:         %d\n", len(toKeep))
		fmt.Printf("  Removing:        %d\n", len(toRemove))
		fmt.Println("")

		if len(toRemove) == 0 {
			fmt.Println("Nothing to remove. All releases are within the keep limit.")
			return nil
		}

		// Show what will be removed
		fmt.Println("Releases to remove:")
		for _, version := range toRemove {
			versionPath := filepath.Join(releasesDir, version)
			size, _ := getDirSize(versionPath)
			fmt.Printf("  - %s (%s)\n", version, formatBytes(size))
		}
		fmt.Println("")

		// Dry run mode
		if cleanupDry {
			fmt.Println("Dry run mode: No files were removed.")
			fmt.Println("Run without --dry-run to actually remove these releases.")
			return nil
		}

		// Confirm removal
		fmt.Printf("Remove %d old release(s)? [y/N]: ", len(toRemove))
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Cleanup cancelled.")
			return nil
		}

		// Remove old versions
		removedCount := 0
		for _, version := range toRemove {
			versionPath := filepath.Join(releasesDir, version)
			logger.Info("Removing old release", "version", version)
			if err := os.RemoveAll(versionPath); err != nil {
				logger.Warning("Failed to remove old release", "version", version, "error", err)
			} else {
				removedCount++
			}
		}

		fmt.Printf("\n✓ Removed %d old release(s)\n", removedCount)
		fmt.Printf("  Kept: %s\n", strings.Join(toKeep, ", "))
		return nil
	},
}

// getDirSize calculates the total size of a directory recursively
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func init() {
	NativeCmd.AddCommand(cleanupCmd)

	cleanupCmd.Flags().IntVarP(&cleanupKeep, "keep", "k", 3, "Number of recent releases to keep (default: 3)")
	cleanupCmd.Flags().BoolVar(&cleanupDry, "dry-run", false, "Show what would be removed without actually removing")
}
