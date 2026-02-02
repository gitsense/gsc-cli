/**
 * Component: Doctor Logic
 * Block-UUID: 9cd2ccfc-4328-41e0-b33e-6c99dcde32c7
 * Parent-UUID: 3dba2ac2-c61d-4822-8baf-2d98c047df0e
 * Version: 1.1.1
 * Description: Logic to perform health checks on the .gitsense environment, including directory, registry, and database validation. Removed unused ValidateRegistryJSON function.
 * Language: Go
 * Created-at: 2026-02-02T08:32:35.212Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), Claude Haiku 4.5 (v1.1.1)
 */


package manifest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/internal/registry"
)

// DoctorReport represents the result of a health check.
type DoctorReport struct {
	IsHealthy bool           `json:"is_healthy"`
	Checks    []CheckResult  `json:"checks"`
}

// CheckResult represents the result of a single health check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "warning", "error"
	Message string `json:"message"`
}

// RunDoctor performs a series of health checks on the .gitsense environment.
func RunDoctor(ctx context.Context, fix bool) (*DoctorReport, error) {
	report := &DoctorReport{
		IsHealthy: true,
		Checks:    []CheckResult{},
	}

	// 1. Check Project Root
	root, err := git.FindProjectRoot()
	if err != nil {
		report.IsHealthy = false
		report.Checks = append(report.Checks, CheckResult{
			Name:    "Project Root",
			Status:  "error",
			Message: fmt.Sprintf("Not in a Git repository: %v", err),
		})
		return report, nil
	}
	report.Checks = append(report.Checks, CheckResult{
		Name:    "Project Root",
		Status:  "ok",
		Message: fmt.Sprintf("Found at %s", root),
	})

	gitsenseDir := filepath.Join(root, settings.GitSenseDir)

	// 2. Check .gitsense Directory
	if _, err := os.Stat(gitsenseDir); os.IsNotExist(err) {
		report.IsHealthy = false
		report.Checks = append(report.Checks, CheckResult{
			Name:    "GitSense Directory",
			Status:  "error",
			Message: fmt.Sprintf("Directory not found at %s", gitsenseDir),
		})
		return report, nil
	}
	report.Checks = append(report.Checks, CheckResult{
		Name:    "GitSense Directory",
		Status:  "ok",
		Message: fmt.Sprintf("Found at %s", gitsenseDir),
	})

	// 3. Check Registry File
	registryPath := filepath.Join(gitsenseDir, settings.RegistryFileName)
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		report.IsHealthy = false
		report.Checks = append(report.Checks, CheckResult{
			Name:    "Registry File",
			Status:  "error",
			Message: fmt.Sprintf("File not found at %s", registryPath),
		})
		return report, nil
	}

	// Try to parse registry
	reg, err := registry.LoadRegistry()
	if err != nil {
		report.IsHealthy = false
		report.Checks = append(report.Checks, CheckResult{
			Name:    "Registry File",
			Status:  "error",
			Message: fmt.Sprintf("Failed to parse: %v", err),
		})
		return report, nil
	}
	report.Checks = append(report.Checks, CheckResult{
		Name:    "Registry File",
		Status:  "ok",
		Message: fmt.Sprintf("Loaded successfully, %d databases registered", len(reg.Databases)),
	})

	// 4. Check Database Connectivity
	for _, entry := range reg.Databases {
		dbPath := filepath.Join(gitsenseDir, entry.Name+".db")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			report.IsHealthy = false
			report.Checks = append(report.Checks, CheckResult{
				Name:    fmt.Sprintf("Database: %s", entry.Name),
				Status:  "error",
				Message: "Database file missing",
			})
			continue
		}

		// Try to open database
		database, err := db.OpenDB(dbPath)
		if err != nil {
			report.IsHealthy = false
			report.Checks = append(report.Checks, CheckResult{
				Name:    fmt.Sprintf("Database: %s", entry.Name),
				Status:  "error",
				Message: fmt.Sprintf("Failed to connect: %v", err),
			})
			continue
		}
		db.CloseDB(database)

		report.Checks = append(report.Checks, CheckResult{
			Name:    fmt.Sprintf("Database: %s", entry.Name),
			Status:  "ok",
			Message: "Connection successful",
		})
	}

	// 5. Check for Orphaned Files
	entries, err := os.ReadDir(gitsenseDir)
	if err != nil {
		report.IsHealthy = false
		report.Checks = append(report.Checks, CheckResult{
			Name:    "Orphan Check",
			Status:  "error",
			Message: fmt.Sprintf("Failed to read directory: %v", err),
		})
	} else {
		orphanCount := 0
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			// Check if it's a .db file
			if strings.HasSuffix(name, ".db") {
				dbName := strings.TrimSuffix(name, ".db")
				
				// Check if it's in the registry
				found := false
				for _, regEntry := range reg.Databases {
					if regEntry.Name == dbName {
						found = true
						break
					}
				}

				if !found {
					orphanCount++
					report.IsHealthy = false
					report.Checks = append(report.Checks, CheckResult{
						Name:    "Orphaned Database",
						Status:  "warning",
						Message: fmt.Sprintf("Found unregistered database: %s", name),
					})
				}
			}
		}

		if orphanCount == 0 {
			report.Checks = append(report.Checks, CheckResult{
				Name:    "Orphan Check",
				Status:  "ok",
				Message: "No orphaned database files found",
			})
		}
	}

	return report, nil
}
