/**
 * Component: Native App CLI Package
 * Block-UUID: 6588fb9b-eb8b-48c9-bb16-7536dab2c7ad
 * Parent-UUID: 979b59db-4766-401f-81ae-5d0f3d15e8b5
 * Version: 1.3.0
 * Description: Added support for .gitignore-style glob patterns (**) using doublestar library. Updated exclusions to include recursive patterns for gscb and enterprise directories. Fixed "write too long" error by explicitly skipping the output tarball file during the directory walk to prevent self-referential packaging.
 * Language: Go
 * Created-at: 2026-05-12T15:09:49.437Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.1.1), GLM-4.7 (v1.2.0), GLM-4.7 (v1.2.1), GLM-4.7 (v1.3.0)
 */


package native

import (
	"github.com/AlecAivazis/survey/v2"
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	pkgVersion string
	pkgOutput  string
	pkgAppDir  string
	pkgPrefix  string
)

var packageCmd = &cobra.Command{
	Use:   "package",
	Short: "Package the GitSense Chat source code into a distributable tarball",
	Long: `Creates a .tar.gz archive of the GitSense Chat source code, suitable for
distribution or testing with 'gsc app native install --tarball'.

The archive is structured to match GitHub's auto-generated archives, with all
files wrapped in a versioned prefix directory (e.g., chat-0.1.0/).

Common exclusions are applied automatically (node_modules, .git, docker, etc.).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// Step 1: Resolve version
		version := pkgVersion
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}

		// Step 2: Resolve app directory
		appDir := pkgAppDir
		if appDir == "" {
			appDir = "."
		}
		absAppDir, err := filepath.Abs(appDir)
		if err != nil {
			return fmt.Errorf("failed to resolve app directory: %w", err)
		}

		// Step 3: Resolve prefix
		prefix := pkgPrefix
		if prefix == "" {
			prefix = "chat-" + strings.TrimPrefix(version, "v") + "/"
		}
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}

		// Step 4: Resolve output path
		outputPath := pkgOutput
		if outputPath == "" {
			outputPath = fmt.Sprintf("gitsense-chat-%s.tar.gz", version)
		}
		absOutputPath, err := filepath.Abs(outputPath)
		if err != nil {
			return fmt.Errorf("failed to resolve output path: %w", err)
		}

		// Step 5: Show summary
		fmt.Println("\nGitSense Chat Package Summary")
		fmt.Println("------------------------------")
		fmt.Printf("  Version:  %s\n", version)
		fmt.Printf("  App Dir:  %s\n", absAppDir)
		fmt.Printf("  Prefix:   %s\n", prefix)
		fmt.Printf("  Output:   %s\n", absOutputPath)
		fmt.Println("")

		// Step 6: Confirm before proceeding
		confirm := false
		if err := survey.AskOne(&survey.Confirm{
			Message: "Proceed with packaging?",
			Default: true,
		}, &confirm); err != nil {
			return fmt.Errorf("confirmation prompt failed: %w", err)
		}
		if !confirm {
			fmt.Println("Packaging cancelled.")
			return nil
		}

		// Step 6: Create tarball
		logger.Info("Creating tarball...", "output", absOutputPath)
		if err := createTarball(absAppDir, absOutputPath, prefix); err != nil {
			return fmt.Errorf("failed to create tarball: %w", err)
		}

		logger.Success("Package created", "path", absOutputPath)
		fmt.Println("\nYou can now install this package with:")
		fmt.Printf("  gsc app native install --version %s --tarball %s\n", version, absOutputPath)

		return nil
	},
}

// Exclusions are directories/files to skip when packaging
// Supports glob patterns including ** for recursive matching
var exclusions = []string{
	"**/.DS_Store/**",
	"**/.env/**/",
	"**/.git/**/",
	"**/.gitsense/**/",
	"**/enterprise/**",
	"**/gscb/**",
	"**/node_modules/**",
	"active",
	"bin/dist",
	"bin/gsc",
	"chat.app",
	"data",
	"docker",
	"minify-repo",
	"releases",
	"scripts",
	".minifyexclude",
	".minifyignore",
}

// createTarball creates a gzipped tar archive of srcDir with the specified prefix
func createTarball(srcDir, outputPath, prefix string) error {
	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Create gzip writer
	gzw := gzip.NewWriter(outFile)
	defer gzw.Close()

	// Create tar writer with PAX format to support long filenames
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// Walk the source directory
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from source directory
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Skip the output file itself if it's inside the source directory
		// This prevents self-referential packaging which causes "write too long" errors
		if path == outputPath {
			return nil
		}

		// Check exclusions using glob patterns
		for _, pattern := range exclusions {
			matched, _ := doublestar.Match(pattern, relPath)
			if matched {
				if info.IsDir() {
					// Skip the entire directory
					return filepath.SkipDir
				}
				// Skip this file
				return nil
			}
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Set the name with prefix
		header.Name = filepath.ToSlash(filepath.Join(prefix, relPath))

		// CRITICAL FIX: Use PAX format to support long filenames
		// Standard tar format has a 100-byte limit for filenames
		// PAX format supports arbitrarily long filenames
		header.Format = tar.FormatPAX

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If it's a regular file, write its content
		if !info.IsDir() && info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tw, file)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func init() {
	NativeCmd.AddCommand(packageCmd)

	packageCmd.Flags().StringVar(&pkgVersion, "version", "", "Version tag (e.g., v0.1.0)")
	packageCmd.Flags().StringVar(&pkgOutput, "output", "", "Output path for the tarball (default: ./gitsense-chat-<version>.tar.gz)")
	packageCmd.Flags().StringVar(&pkgAppDir, "app-dir", "", "Source directory to package (default: current directory)")
	packageCmd.Flags().StringVar(&pkgPrefix, "prefix", "", "Directory prefix inside the tarball (default: chat-<version>/)")

	_ = packageCmd.MarkFlagRequired("version")
}
