/**
 * Component: Native App CLI Install
 * Block-UUID: 38634b32-4ce3-4bdb-9da1-345a58e3d9f1
 * Parent-UUID: 09342fee-29de-4a27-a1e4-544029790b90
 * Version: 1.13.0
 * Description: Enhanced installation UX with GSC_HOME handling, improved directory summary, and clearer next steps. Now ignores GSC_HOME during install to prevent path duplication, and provides comprehensive post-install guidance including Claude Code CLI integration and admin llm commands.
 * Language: Go
 * Created-at: 2026-05-30T02:16:39.763Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.3.2), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.8.1), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0)
 */


package native

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gitsense/gsc-cli/internal/native"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

var (
	installAppDir    string
	installDataDir   string
	installVersion   string
	installForce     bool
	installQuiet     bool
	installSkipNPM   bool
	installSkipSetup bool
	installTarball   string
)

var installStartTime time.Time

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the GitSense Chat application natively",
	Long: `Downloads the GitSense Chat Node.js source archive from GitHub releases,
extracts it to a versioned staging directory, copies it to the active app
directory, runs npm install, and initializes the data directory.
	
On a fresh install (no existing database), gsc-admin-setup is run automatically.
On an upgrade, the existing data directory is left untouched.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// Step 1: Normalize and validate the version flag
		version := installVersion
		installStartTime = time.Now()
		
		// Handle "latest" version
		if version == "latest" {
			logger.Info("Fetching latest release from GitHub...")
			latestVersion, err := getLatestRelease()
			if err != nil {
				return fmt.Errorf("failed to get latest release: %w", err)
			}
			version = latestVersion
			logger.Info("Latest release", "version", version)
		}

		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}

		// Capture GSC_HOME environment variable early
		envGSCHome := os.Getenv("GSC_HOME")

		// Step 2: Resolve all paths
		// ALWAYS use user home directory as base for install, ignoring GSC_HOME
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to resolve user home directory: %w", err)
		}

		gscHome := filepath.Join(homeDir, ".gitsense")

		stagedDir := filepath.Join(gscHome, "releases", version, "app")

		appDir := installAppDir
		if appDir == "" {
			appDir = filepath.Join(gscHome, "active", "app")
		} else {
			abs, err := filepath.Abs(appDir)
			if err != nil {
				return fmt.Errorf("failed to resolve --app-dir: %w", err)
			}
			appDir = abs
		}

		dataDir := installDataDir
		if dataDir == "" {
			dataDir = filepath.Join(gscHome, settings.AppDataDirRelPath)
		} else {
			abs, err := filepath.Abs(dataDir)
			if err != nil {
				return fmt.Errorf("failed to resolve --data-dir: %w", err)
			}
			dataDir = abs
		}

		// Step 3: Check prerequisites
		logger.Info("Checking prerequisites...")
		if _, err := exec.LookPath("node"); err != nil {
			return fmt.Errorf("node not found in PATH. Install Node.js from https://nodejs.org/")
		}
		if _, err := exec.LookPath("npm"); err != nil {
			return fmt.Errorf("npm not found in PATH. Install Node.js from https://nodejs.org/")
		}
		if _, err := exec.LookPath("git"); err != nil {
			logger.Warning("git not found in PATH - gsc-admin-setup may fail if it requires git")
		}

		// Step 4: Determine if a cached staged copy is available
		_, statErr := os.Stat(stagedDir)
		alreadyStaged := statErr == nil

		// Step 4.5: Validate data directory location (BEFORE confirmation)
		// Check if dataDir is inside appDir (unsafe for upgrades)
		relPath, err := filepath.Rel(appDir, dataDir)
		if err == nil && !strings.HasPrefix(relPath, "..") {
			// dataDir is inside appDir - this is unsafe for upgrades
			return fmt.Errorf(
				"data directory cannot be inside the app directory\n\n"+
					"  App directory:  %s\n"+
					"  Data directory: %s\n\n"+
					"This configuration is unsafe because upgrades would delete your data.\n"+
					"The app directory is replaced on every upgrade, but the data directory\n"+
					"must persist across upgrades.\n\n"+
					"Solution: Specify a separate data directory outside the app directory.\n"+
					"  Example: gsc app native install --version v0.1.0 --app-dir %s --data-dir %s\n\n"+
					"A symlink will be created from $GSC_HOME/data to your data directory,\n"+
					"so the app can still access it at $GSC_HOME/data.",
				appDir, dataDir,
				appDir, filepath.Join(filepath.Dir(appDir), "data"),
			)
		}

		// Step 5: Show installation summary and prompt for confirmation
		printInstallHeader()
		fmt.Println("------------------------------------------")
		fmt.Printf("  Version:   %s\n", version)
		fmt.Printf("  App Dir:   %s\n", appDir)
		fmt.Printf("  Data Dir:  %s\n", dataDir)
		
		// GSC_HOME status for confirmation
		var gscHomeDisplay string
		if envGSCHome == "" {
			gscHomeDisplay = "Not set (Clean Install)"
		} else {
			gscHomeDisplay = "Ignored during install (will be set to App Dir after install)"
		}
		fmt.Printf("  GSC_HOME:  %s\n", gscHomeDisplay)
		
		if installTarball != "" {
			fmt.Printf("  Source:    %s (local tarball)\n", installTarball)
		} else if alreadyStaged && !installForce {
			fmt.Printf("  Source:    %s (cached)\n", stagedDir)
		} else {
			fmt.Printf("  Source:    https://github.com/gitsense/chat/archive/refs/tags/%s.tar.gz\n", version)
		}
		fmt.Println("")

		confirm := false
		if err := survey.AskOne(&survey.Confirm{
			Message: "Proceed with installation?",
			Default: true,
		}, &confirm); err != nil {
			return fmt.Errorf("confirmation prompt failed: %w", err)
		}
		if !confirm {
			fmt.Println("Installation cancelled.")
			return nil
		}

		printPhaseHeader("Phase 1: Download & Extract")
		// Step 6: Download and extract source archive
		if installTarball != "" {
			// Use local tarball (for testing)
			logger.Info("Using local tarball...", "path", installTarball)

			// Validate the tarball exists
			if _, err := os.Stat(installTarball); os.IsNotExist(err) {
				return fmt.Errorf("tarball file not found: %s", installTarball)
			}

			// Extract directly to staged directory
			if err := os.RemoveAll(stagedDir); err != nil {
				return fmt.Errorf("failed to clear staged directory: %w", err)
			}
			if err := extractTarGzStripped(installTarball, stagedDir, version); err != nil {
				_ = os.RemoveAll(stagedDir)
				return fmt.Errorf("extraction failed: %w", err)
			}
			printStepSuccess("Extracting archive to staging directory", stagedDir)
		} else if !alreadyStaged || installForce {
			// Download from GitHub
			url := fmt.Sprintf("https://github.com/gitsense/chat/archive/refs/tags/%s.tar.gz", version)
			printStepInfo("Downloading source archive", fmt.Sprintf("v%s from GitHub", version))

			tmpFile, err := os.CreateTemp("", "gsc-install-*.tar.gz")
			if err != nil {
				return fmt.Errorf("failed to create temp file: %w", err)
			}
			tmpPath := tmpFile.Name()
			defer os.Remove(tmpPath)

			if err := downloadToFile(url, tmpFile); err != nil {
				tmpFile.Close()
				return fmt.Errorf("download failed: %w", err)
			}
			tmpFile.Close()

			logger.Info("Extracting archive...", "dest", stagedDir)
			if err := os.RemoveAll(stagedDir); err != nil {
				return fmt.Errorf("failed to clear staged directory: %w", err)
			}
			if err := extractTarGzStripped(tmpPath, stagedDir, version); err != nil {
				_ = os.RemoveAll(stagedDir)
				return fmt.Errorf("extraction failed: %w", err)
			}
			printStepSuccess("Extracting archive to staging directory", stagedDir)
		} else {
			printStepInfo("Using cached staged source", stagedDir)
		}

		// Step 7: Deploy staged → active (never touches dataDir)
		printStepInfo("Deploying to active directory", appDir)
		if err := os.RemoveAll(appDir); err != nil {
			return fmt.Errorf("failed to clear active app directory: %w", err)
		}
		if err := copyDirAll(stagedDir, appDir); err != nil {
			return fmt.Errorf("failed to copy to active directory: %w", err)
		}
		printStepSuccess("Deploying to active directory", appDir)

		// Step 7.5: Create bin/gsc symlink to the actual gsc binary in PATH
		printStepInfo("Creating bin/gsc symlink", "")
		gscBinaryPath, err := exec.LookPath("gsc")
		if err != nil {
			logger.Warning("gsc binary not found in PATH", "error", err)
			logger.Warning("The chat app may not be able to find the gsc CLI")
			logger.Warning("You can manually create the symlink later:")
			logger.Warning(fmt.Sprintf("  ln -s $(which gsc) %s/bin/gsc", appDir))
		} else {
			symlinkPath := filepath.Join(appDir, "bin", "gsc")
			_ = os.Remove(symlinkPath)
			// Create symlink to the actual gsc binary
			if err := os.Symlink(gscBinaryPath, symlinkPath); err != nil {
				logger.Warning("Failed to create bin/gsc symlink", "error", err)
			} else {
				printStepSuccess("Creating bin/gsc symlink", fmt.Sprintf("%s → %s", symlinkPath, gscBinaryPath))
			}
		}

		// Step 7.6: Create data symlink in app directory
		printStepInfo("Creating data symlink", "")
		dataSymlinkPath := filepath.Join(appDir, "data")
		
		// Remove existing data directory or symlink (from tarball or previous install)
		_ = os.RemoveAll(dataSymlinkPath)
		
		// Create symlink to actual data directory
		// Use relative path for portability: active/app/data -> ../../app/data
		relDataPath, err := filepath.Rel(appDir, dataDir)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path for data symlink: %w", err)
		}
		
		if err := os.Symlink(relDataPath, dataSymlinkPath); err != nil {
			return fmt.Errorf("failed to create data symlink: %w", err)
		}
		printStepSuccess("Creating data symlink", fmt.Sprintf("%s → %s", dataSymlinkPath, relDataPath))
		
		printPhaseHeader("Phase 2: Install Dependencies")
		// Step 8: Run npm install
		if !installSkipNPM {
			printStepInfo("Running npm install", "this may take a few minutes")
			npmCmd := exec.Command("npm", "install", "--omit=dev")
			npmCmd.Dir = appDir
			npmCmd.Stdout = os.Stdout
			npmCmd.Stderr = os.Stderr
			if err := npmCmd.Run(); err != nil {
				return fmt.Errorf("npm install failed: %w", err)
			}
			printStepSuccess("Running npm install", "complete")
		}

		printPhaseHeader("Phase 3: Initialize Data")
		// Step 9: Create data directory structure
		if err := os.MkdirAll(filepath.Join(dataDir, "logs"), 0755); err != nil {
			return fmt.Errorf("failed to create data directories: %w", err)
		}
		printStepSuccess("Creating data directories", filepath.Join(dataDir, "logs"))

		// Step 10: Run gsc-admin-setup only on a fresh install (no existing DB)
		if !installSkipSetup {
			dbPath := filepath.Join(dataDir, "chats.sqlite3")
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				printStepInfo("Initializing database", "from base-state")
				setupScript := filepath.Join(appDir, "bin", "gsc-admin-setup")
				if _, err := os.Stat(setupScript); os.IsNotExist(err) {
					return fmt.Errorf("gsc-admin-setup not found at %s", setupScript)
				}
				setupCmd := exec.Command("node", setupScript, "--data-dir", dataDir, "--force")
				// cmd.Dir is critical: gsc-admin-setup resolves base-state via __dirname
				setupCmd.Dir = appDir
				setupCmd.Stdout = os.Stdout
				setupCmd.Stderr = os.Stderr
				if err := setupCmd.Run(); err != nil {
					return fmt.Errorf("gsc-admin-setup failed: %w", err)
				}
				printStepSuccess("Initializing database", "complete")
			} else {
				printStepInfo("Existing data directory found", "skipping setup (upgrade path)")
			}
		}

		// Step 11: Seed .env from .env.example if no .env exists yet
		envPath := filepath.Join(dataDir, ".env")
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			examplePath := filepath.Join(appDir, ".env.example")
			if _, err := os.Stat(examplePath); err == nil {
				if err := copyFileSingle(examplePath, envPath); err != nil {
					logger.Warning("Failed to copy .env.example", "error", err)
				} else {
					printStepSuccess("Creating .env from template", envPath)
				}
			}
		}

		printPhaseHeader("Phase 4: Save Configuration")
		// Step 12: Write native-config.json
		cfg := native.Config{
			Version:     version,
			AppDir:      appDir,
			DataDir:     dataDir,
			Port:        settings.DefaultAppPort,
			InstalledAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := native.SaveConfig(gscHome, cfg); err != nil {
			return fmt.Errorf("failed to write native-config.json: %w", err)
		}
		printStepSuccess("Saving configuration", filepath.Join(gscHome, "native-config.json"))

		// Step 13: Print summary and next steps
		printInstallSummary(version, appDir, dataDir, envGSCHome)
		fmt.Println("")
		printDirectorySummary(gscHome, appDir, dataDir)
		fmt.Println("")
		printNextSteps(appDir, envPath, envGSCHome)
		fmt.Println("")

		return nil
	},
}

// downloadToFile streams an HTTP GET response body into an open file.
func downloadToFile(url string, dest *os.File) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url) //nolint:gosec // URL is constructed from a validated version string
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %s for %s", resp.Status, url)
	}

	_, err = io.Copy(dest, resp.Body)
	return err
}

// getLatestRelease fetches the latest release tag from GitHub API
func getLatestRelease() (string, error) {
	url := "https://api.github.com/repos/gitsense/chat/releases/latest"
	
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	
	var release struct {
		TagName string `json:"tag_name"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse GitHub API response: %w", err)
	}
	
	return release.TagName, nil
}

// extractTarGzStripped extracts a .tar.gz archive into destDir, stripping the
// GitHub-generated top-level prefix (e.g., "chat-0.1.0/").
//
// GitHub omits the 'v' prefix from tag names in archive directory names:
//   tag v0.1.0  →  directory "chat-0.1.0/"
func extractTarGzStripped(src, destDir, version string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	versionStripped := strings.TrimPrefix(version, "v")
	prefix := "chat-" + versionStripped + "/"
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if !strings.HasPrefix(hdr.Name, prefix) {
			continue
		}
		relPath := strings.TrimPrefix(hdr.Name, prefix)
		if relPath == "" {
			continue
		}

		target := filepath.Join(destDir, filepath.FromSlash(relPath))

		// Guard against zip-slip path traversal
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) {
			return fmt.Errorf("invalid path in archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			out.Close()
			if copyErr != nil {
				return copyErr
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyDirAll recursively copies src into dest, preserving file modes.
func copyDirAll(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		// Check if this is a symbolic link
		if info.Mode()&os.ModeSymlink != 0 {
			// Read the link target
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink %s: %w", path, err)
			}
			// Create the symlink in the destination (preserve as symlink)
			return os.Symlink(linkTarget, target)
		}

		return copyFileSingle(path, target)
	})
}

// copyFileSingle copies a single file from src to dest, preserving file permissions.
func copyFileSingle(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func init() {
	NativeCmd.AddCommand(installCmd)

	installCmd.Flags().StringVar(&installVersion, "version", "latest", "Version tag to install (e.g., v0.1.0, or 'latest')")
	installCmd.Flags().StringVar(&installTarball, "tarball", "", "Path to a local tarball file to install from (skips GitHub download)")
	installCmd.Flags().StringVar(&installAppDir, "app-dir", "", "Override the active app directory (default: $GSC_HOME/active/app)")
	installCmd.Flags().StringVar(&installDataDir, "data-dir", "", "Override the persistent data directory (default: $GSC_HOME/app/data)")
	installCmd.Flags().BoolVar(&installForce, "force", false, "Re-download and reinstall even if the version is already staged")
	installCmd.Flags().BoolVar(&installSkipNPM, "skip-npm", false, "Skip the npm install step")
	installCmd.Flags().BoolVar(&installSkipSetup, "skip-setup", false, "Skip gsc-admin-setup even on a fresh install")
}

// --- UX Helper Functions ---

func printInstallHeader() {
	fmt.Println("\n" + strings.Repeat("━", 60))
	fmt.Println("  GitSense Chat Native Installation")
	fmt.Println(strings.Repeat("━", 60))
}

func printPhaseHeader(phase string) {
	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Printf("  %s\n", phase)
	fmt.Println(strings.Repeat("─", 60))
}

func printStepInfo(step, detail string) {
	if detail != "" {
		fmt.Printf("  → %s (%s)...\n", step, detail)
	} else {
		fmt.Printf("  → %s...\n", step)
	}
}

func printStepSuccess(step, detail string) {
	if detail != "" {
		fmt.Printf("  ✓ %s (%s)\n", step, detail)
	} else {
		fmt.Printf("  ✓ %s\n", step)
	}
}

func printInstallSummary(version, appDir, dataDir, envGSCHome string) {
	duration := time.Since(installStartTime)
	
	fmt.Println("\n" + strings.Repeat("━", 60))
	fmt.Println("  Installation Summary")
	fmt.Println(strings.Repeat("━", 60))
	fmt.Printf("  Version:        %s\n", version)
	fmt.Printf("  App Directory:  %s\n", appDir)
	fmt.Printf("  Data Directory: %s\n", dataDir)
	
	// GSC_HOME status
	if envGSCHome == "" {
		fmt.Printf("  GSC_HOME:       %s (set this in your shell)\n", appDir)
	} else {
		fmt.Printf("  GSC_HOME:       %s ✓ (set in environment)\n", envGSCHome)
	}
	
	// Try to get npm package count
	packageCount := "N/A"
	if packageJSONPath := filepath.Join(appDir, "package.json"); fileExists(packageJSONPath) {
		if nodeModulesPath := filepath.Join(appDir, "node_modules"); dirExists(nodeModulesPath) {
			// Count directories in node_modules (rough estimate)
			entries, err := os.ReadDir(nodeModulesPath)
			if err == nil {
				packageCount = fmt.Sprintf("%d packages", len(entries))
			}
		}
	}
	fmt.Printf("  Dependencies:   %s\n", packageCount)
	
	// Check database status
	dbStatus := "Not initialized"
	if dbPath := filepath.Join(dataDir, "chats.sqlite3"); fileExists(dbPath) {
		dbStatus = "Initialized"
	}
	fmt.Printf("  Database:       %s\n", dbStatus)
	
	// Check .env status
	envStatus := "Not created"
	if envPath := filepath.Join(dataDir, ".env"); fileExists(envPath) {
		envStatus = "Created"
	}
	fmt.Printf("  Configuration:  %s\n", envStatus)
	
	// Format duration
	durationStr := formatDuration(duration)
	fmt.Printf("  Duration:       %s\n", durationStr)
	fmt.Println(strings.Repeat("━", 60))
	fmt.Println("  ✓ Installation complete!")
	fmt.Println(strings.Repeat("━", 60))
}

func printDirectorySummary(gscHome, appDir, dataDir string) {
	// Helper to shorten home directory paths
	shortenPath := func(path string) string {
		homeDir, _ := os.UserHomeDir()
		if strings.HasPrefix(path, homeDir) {
			return "~" + path[len(homeDir):]
		}
		return path
	}

	fmt.Println("Directory Summary:")
	
	// Show GSC_HOME (app directory)
	fmt.Printf("  %s/          ← GSC_HOME (app directory with bin/ and data/ symlinks)\n", shortenPath(appDir))
	fmt.Println("  ├── bin/                ← gsc binary symlink")
	
	// Show data symlink
	relDataPath, _ := filepath.Rel(appDir, dataDir)
	if relDataPath == "." {
		fmt.Printf("  ├── data/              ← Persistent data (DO NOT DELETE)\n")
	} else {
		fmt.Printf("  ├── data → %s/  ← Symlink to persistent data\n", relDataPath)
	}
	fmt.Println("  └── index.js            ← Application entry point")
	
	// Show actual data directory
	fmt.Printf("  %s/          ← Persistent data (DO NOT DELETE)\n", shortenPath(dataDir))
	fmt.Println("  ├── chats.sqlite3")
	fmt.Println("  ├── .env")
	fmt.Println("  └── logs/")
	
	// Show gscHome directories (always in ~/.gitsense)
	fmt.Printf("  %s/    ← Cached archives (safe to delete, but upgrades will be slower)\n", shortenPath(filepath.Join(gscHome, "releases")))
	fmt.Printf("  %s/   ← CLI templates (safe to delete, will be recreated)\n", shortenPath(filepath.Join(gscHome, "cli", "templates")))
	fmt.Printf("  %s   ← Installation config (DO NOT DELETE)\n", shortenPath(filepath.Join(gscHome, "native-config.json")))
}

func printNextSteps(appDir, envPath, envGSCHome string) {
	stepNum := 1
	fmt.Println("Next Steps:")

	rcFile := "~/.zshrc"
	if strings.Contains(os.Getenv("SHELL"), "bash") {
		rcFile = "~/.bashrc"
	}
	fmt.Printf("  %d. Set GSC_HOME environment variable (required):\n", stepNum)
	fmt.Printf("     export GSC_HOME=%s\n", appDir)
	fmt.Printf("     echo 'export GSC_HOME=%s' >> %s\n", appDir, rcFile)
	fmt.Println("")
	stepNum++

	fmt.Printf("  %d. Configure your API keys:\n", stepNum)
	fmt.Println("     Edit the .env file to add your API keys:")
	fmt.Printf("       %s\n", envPath)
	fmt.Println("")
	fmt.Println("     Or use the admin command to manage providers:")
	fmt.Println("       gsc app native admin llm list providers")
	fmt.Println("       gsc app native admin llm edit provider")
	fmt.Println("")
	stepNum++

	fmt.Printf("  %d. Manage LLM models:\n", stepNum)
	fmt.Println("     gsc app native admin llm list models")
	fmt.Println("     gsc app native admin llm add model")
	fmt.Println("")
	stepNum++

	fmt.Printf("  %d. (Optional) Enable Claude Code CLI integration:\n", stepNum)
	fmt.Println("     gsc claude init")
	fmt.Println("")
	stepNum++

	fmt.Printf("  %d. Start the application:\n", stepNum)
	fmt.Println("     gsc app native start")
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%dm %.1fs", int(d.Minutes()), d.Seconds()-float64(int(d.Minutes()))*60)
	} else {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
