/**
 * Component: Import Git Progress UI
 * Block-UUID: 14b2505c-610c-49c5-9f6f-d56b7eedd504
 * Parent-UUID: d5a4c2ef-5e40-4d38-94ce-8a7945fad074
 * Version: 2.7.0
 * Description: Updated gitignore warning message to use centralized gitignore service. Changed from manual 'echo' command suggestion to 'gsc gitignore update' command for consistent pattern management across all GitSense features.
 * Language: Go
 * Created-at: 2026-05-23T15:09:51.506Z
 * Authors: ..., GLM-4.7 (v2.2.0), GLM-4.7 (v2.3.0), Gemini 2.5 Flash Lite (v2.4.0), GLM-4.7 (v2.5.0), GLM-4.7 (v2.5.1), Gemini 2.5 Flash Lite (v2.5.2), Gemini 2.5 Flash Lite (v2.5.3), Gemini 2.5 Flash Lite (v2.6.0), GLM-4.7 (v2.7.0)
 */


package importgit

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gitsense/gsc-cli/internal/output"
)

// ProgressUI manages the display of import progress
type ProgressUI struct {
	totalFiles      int
	doneCount       int
	skippedCount    int
	skippedReasons  map[string]int
	currentFile     string
	startTime       time.Time
	refChatID       int64
	durationMs      int
	spinnerIdx      int
	spinnerFrames   []rune
	isTerminal      bool
	width           int
	isShadowUpdate  bool
	shadowStartTime time.Time
	numLines        int
	// v2.0.1: Changed from time.Duration to time.Time to track start times
	phaseTimings    map[ShadowPhase]time.Time
	// v2.0.3: Added separate map to store actual phase durations
	phaseDurations  map[ShadowPhase]time.Duration
	currentPhase    ShadowPhase
	// v2.0.3: Track number of lines printed by shadow progress for proper cleanup
	shadowNumLines  int
	// v2.2.0: Added mutex to protect concurrent access to phaseTimings and phaseDurations
	mu              sync.RWMutex
}

// NewProgressUI creates a new ProgressUI instance
func NewProgressUI() *ProgressUI {
	return &ProgressUI{
		skippedReasons: make(map[string]int),
		spinnerFrames:  []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'},
		startTime:      time.Now(),
		isTerminal:     output.IsTerminal(),
		width:          output.GetTerminalWidth(),
		phaseTimings:   make(map[ShadowPhase]time.Time),
		phaseDurations: make(map[ShadowPhase]time.Duration),
	}
}

// Update processes an NDJSON event and updates the internal state
func (p *ProgressUI) Update(event *NDJSONEvent) {
	switch event.Type {
	case "init":
		// Initialize display
		p.renderLine("Initializing...")
	case "scan_complete":
		var data DataScanComplete
		if err := unmarshalData(event.Data, &data); err == nil {
			p.totalFiles = data.TotalFiles
		}
	case "file_start":
		var data DataFileStart
		if err := unmarshalData(event.Data, &data); err == nil {
			p.currentFile = data.Path
		}
	case "file_done":
		p.doneCount++
		p.renderProgress()
	case "file_skip":
		p.skippedCount++
		var data DataFileSkip
		if err := unmarshalData(event.Data, &data); err == nil {
			p.skippedReasons[data.Reason]++
		}
		p.renderProgress()
	case "complete":
		var data DataComplete
		if err := unmarshalData(event.Data, &data); err == nil {
			p.refChatID = data.RefChatID
			p.durationMs = data.DurationMs
		}
		p.renderLine("Finalizing...")
	case "error":
		var data DataError
		if err := unmarshalData(event.Data, &data); err == nil {
			fmt.Printf("\nError: %s\n", data.Message)
		}
	}
}

// renderProgress draws the progress bar and spinner with multi-line in-place updates
func (p *ProgressUI) renderProgress() {
	if !p.isTerminal {
		return // Don't render fancy progress if piped
	}

	// v2.5.1: Clear shadow timing breakdown if present
	if p.shadowNumLines > 0 {
		p.clearShadowLines()
		p.shadowNumLines = 0
	}

	// Calculate percentage
	percent := 0.0
	if p.totalFiles > 0 {
		percent = float64(p.doneCount) / float64(p.totalFiles)
	}

	// Build progress bar
	barWidth := 40
	if p.width > 0 {
		barWidth = p.width - 40 // Reserve space for text
		if barWidth < 10 {
			barWidth = 10
		}
	}

	filled := int(float64(barWidth) * percent)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Spinner
	spinner := string(p.spinnerFrames[p.spinnerIdx%len(p.spinnerFrames)])
	p.spinnerIdx++

	// Calculate number of lines to display
	numLines := 1 // Progress bar line
	if p.currentFile != "" {
		numLines++
	}
	if p.skippedCount > 0 {
		numLines++
	}

	// Move cursor up and clear previous lines (if any)
	if p.numLines > 0 {
		for i := 0; i < p.numLines; i++ {
			fmt.Print("\033[F\033[2K") // Move up one line and clear entire line
		}
	}

	// Update tracked line count
	p.numLines = numLines

	// Render progress bar line
	line := fmt.Sprintf("\r\033[K%s Importing... [%s] %d%% (%d/%d)",
		spinner,
		bar,
		int(percent*100),
		p.doneCount,
		p.totalFiles,
	)
	fmt.Println(line)

	// Render current file line
	if p.currentFile != "" {
		maxFileLen := 80
		fileDisplay := p.currentFile
		if len(fileDisplay) > maxFileLen {
			fileDisplay = "..." + fileDisplay[len(fileDisplay)-maxFileLen+3:]
		}
		fmt.Printf("   Current: %s\n", fileDisplay)
	}

	// Render skipped count line
	if p.skippedCount > 0 {
		fmt.Printf("   Skipped: %d\n", p.skippedCount)
	}
}

// renderLine prints a simple status line
func (p *ProgressUI) renderLine(text string) {
	if p.isTerminal {
		// Clear previous multi-line progress if any
		if p.numLines > 0 {
			for i := 0; i < p.numLines; i++ {
				fmt.Print("\033[F\033[2K")
			}
			p.numLines = 0
		}

		// v2.5.1: Clear shadow timing breakdown if present
		if p.shadowNumLines > 0 {
			p.clearShadowLines()
			p.shadowNumLines = 0
		}

		fmt.Printf("\r\033[K%s %s\n", string(p.spinnerFrames[p.spinnerIdx%len(p.spinnerFrames)]), text)
		p.spinnerIdx++
	} else {
		fmt.Println(text)
	}
}

// PrintFinalSummary displays the completion summary
func (p *ProgressUI) PrintFinalSummary(owner, repo, branch, dbPath, stateFile string, duration time.Duration) {
	// Clear the progress lines if in terminal
	if p.isTerminal {
		if p.numLines > 0 {
			for i := 0; i < p.numLines; i++ {
				fmt.Print("\033[F\033[2K")
			}
			p.numLines = 0
		}
	}

	fmt.Println("✓ Import complete")
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Repository: %s/%s (%s)\n", owner, repo, branch)
	fmt.Printf("  Mode:       Full (preserves history)\n")
	fmt.Printf("  Files:      %d processed", p.doneCount)
	
	if p.skippedCount > 0 {
		fmt.Printf(", %d skipped", p.skippedCount)
		// Print breakdown
		reasons := []string{}
		for reason, count := range p.skippedReasons {
			reasons = append(reasons, fmt.Sprintf("%s: %d", reason, count))
		}
		fmt.Printf(" (%s)", strings.Join(reasons, ", "))
	}
	fmt.Println()
	
	fmt.Printf("  Duration:   %s\n", duration.Round(time.Millisecond))
	fmt.Printf("  Database:   %s\n", dbPath)
	fmt.Printf("  State:      %s\n", stateFile)
	
	fmt.Println()
	fmt.Println("Next Steps:")
	fmt.Println("  • Update later: gsc app import git --update")
}

// PrintFinalSummaryWithWarning is a wrapper to include the gitignore warning
func (p *ProgressUI) PrintFinalSummaryWithWarning(owner, repo, branch, dbPath, stateFile string, duration time.Duration, showGitIgnoreWarning bool) {
	p.PrintFinalSummary(owner, repo, branch, dbPath, stateFile, duration)
	
	if showGitIgnoreWarning {
		fmt.Println()
		fmt.Println("Configuration:")
		fmt.Println("  ⚠️  The state file .gitsense/import-git.json is not ignored.")
		fmt.Println("      To prevent committing local state, run:")
		fmt.Println("        gsc gitignore update")
	}
}

// StartShadowPhase prepares the terminal for shadow progress output
func (p *ProgressUI) StartShadowPhase() {
	if p.isTerminal {
		// Clear previous multi-line progress if any
		if p.numLines > 0 {
			for i := 0; i < p.numLines; i++ {
				fmt.Print("\033[F\033[2K")
			}
			p.numLines = 0
		}
	}
	p.isShadowUpdate = false
	p.shadowStartTime = time.Now()
	p.currentPhase = PhaseScanning
	
	// v2.2.0: Lock mutex when initializing maps
	p.mu.Lock()
	p.phaseTimings = make(map[ShadowPhase]time.Time)
	p.phaseDurations = make(map[ShadowPhase]time.Duration)
	p.mu.Unlock()
	
	p.shadowNumLines = 0
}

// StartShadowUpdatePhase prepares the terminal for shadow update progress output
func (p *ProgressUI) StartShadowUpdatePhase() {
	if p.isTerminal {
		// Clear previous multi-line progress if any
		if p.numLines > 0 {
			for i := 0; i < p.numLines; i++ {
				fmt.Print("\033[F\033[2K")
			}
			p.numLines = 0
		}
	}
	p.isShadowUpdate = true
	p.shadowStartTime = time.Now()
	p.currentPhase = PhaseScanning
	
	// v2.2.0: Lock mutex when initializing maps
	p.mu.Lock()
	p.phaseTimings = make(map[ShadowPhase]time.Time)
	p.phaseDurations = make(map[ShadowPhase]time.Duration)
	p.mu.Unlock()
	
	p.shadowNumLines = 0
}

// clearShadowLines erases the lines rendered by the previous shadow progress call.
// Phase lines are printed WITHOUT a trailing newline, so the cursor sits at the
// end of the last content line. \033[F (cursor-previous-line) would jump over that
// line entirely. We therefore clear the current line first with \r\033[2K, then
// move up for each additional line.
func (p *ProgressUI) clearShadowLines() {
	if p.shadowNumLines <= 0 {
		return
	}
	fmt.Print("\r\033[2K") // clear the current line (last line of previous phase)
	for i := 1; i < p.shadowNumLines; i++ {
		fmt.Print("\033[F\033[2K") // move up one line and clear it
	}
}

// ShadowProgressFn returns a callback function compatible with shadow.CreateShadow/UpdateShadow
func (p *ProgressUI) ShadowProgressFn() ShadowProgressFn {
	return func(phase ShadowPhase, current string, copied, total int) {
		if !p.isTerminal {
			return
		}

		p.mu.Lock()

		// Phase timing and transition logic
		if phase != p.currentPhase && p.currentPhase != PhaseDone {
			if startTime, ok := p.phaseTimings[p.currentPhase]; ok {
				p.phaseDurations[p.currentPhase] = time.Since(startTime)
			}
			p.phaseTimings[phase] = time.Now()
			p.currentPhase = phase
		} else if _, ok := p.phaseTimings[phase]; !ok {
			p.phaseTimings[phase] = time.Now()
		}

		var line string
		action := "Creating"
		if p.isShadowUpdate {
			action = "Updating"
		}

		// Clear previous phase output while holding the lock to prevent
		// concurrent goroutines (parallelCopy workers) from interleaving
		// escape sequences and corrupting the terminal display.
		p.clearShadowLines()

		switch phase {
		case PhaseScanning:
			line = fmt.Sprintf("\r%s shadow snapshot...\n  Phase 1/4: Scanning source files... ", action)
			p.shadowNumLines = 2
		case PhaseCopying:
			percent := 0
			if total > 0 {
				percent = int(float64(copied) / float64(total) * 100)
			}
			barWidth := 30
			filled := int(float64(barWidth) * float64(percent) / 100.0)
			bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
			
			line = fmt.Sprintf("\r%s shadow snapshot...\n  Phase 2/4: Copying files... (%d / %d) [%s] %d%%", 
				action, copied, total, bar, percent)
			p.shadowNumLines = 2
		case PhaseStaging:
			line = fmt.Sprintf("\r%s shadow snapshot...\n  Phase 3/4: Staging files in git... (indexing %d files)", action, total)
			p.shadowNumLines = 2
		case PhaseCommitting:
			line = fmt.Sprintf("\r%s shadow snapshot...\n  Phase 4/4: Committing snapshot... (creating commit)", action)
			p.shadowNumLines = 2
		case PhaseDone:
			totalDuration := time.Since(p.shadowStartTime)
			
			scanTime := p.phaseDurations[PhaseScanning]
			copyTime := p.phaseDurations[PhaseCopying]
			stageTime := p.phaseDurations[PhaseStaging]
			commitTime := p.phaseDurations[PhaseCommitting]
			
			line = fmt.Sprintf("\r\033[KShadow snapshot %s ✓  (%d files, %s)\n", 
				strings.ToLower(action), total, totalDuration.Round(time.Millisecond))
			
			// Show detailed timing breakdown
			fmt.Print(line)
			fmt.Println("  Timing breakdown:")
			fmt.Printf("    • Scan:   %s\n", scanTime.Round(time.Millisecond))
			fmt.Printf("    • Copy:   %s\n", copyTime.Round(time.Millisecond))
			fmt.Printf("    • Stage:  %s\n", stageTime.Round(time.Millisecond))
			fmt.Printf("    • Commit: %s\n", commitTime.Round(time.Millisecond))
			fmt.Printf("    • Total:  %s\n", totalDuration.Round(time.Millisecond))
			
			p.shadowNumLines = 8
			p.mu.Unlock()
			return
		}
		fmt.Print(line)
		p.mu.Unlock()
	}
}

// PrintShadowFinalSummary displays the completion summary for shadow imports
func (p *ProgressUI) PrintShadowFinalSummary(owner, repo, branch, dbPath, stateFile, shadowPath string, duration time.Duration) {
	// Clear the progress line if in terminal
	if p.isTerminal {
		fmt.Print("\r\033[K")
	}

	fmt.Println("✓ Shadow import complete")
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Repository: %s/%s (%s)\n", owner, repo, branch)
	fmt.Printf("  Mode:       Shadow (single-commit snapshot)\n")
	fmt.Printf("  Files:      %d processed", p.doneCount)
	
	if p.skippedCount > 0 {
		fmt.Printf(", %d skipped", p.skippedCount)
		reasons := []string{}
		for reason, count := range p.skippedReasons {
			reasons = append(reasons, fmt.Sprintf("%s: %d", reason, count))
		}
		fmt.Printf(" (%s)", strings.Join(reasons, ", "))
	}
	fmt.Println()
	
	fmt.Printf("  Duration:   %s\n", duration.Round(time.Millisecond))
	fmt.Printf("  Database:   %s\n", dbPath)
	fmt.Printf("  State:      %s\n", stateFile)
	
	fmt.Println()
	fmt.Println("Next Steps:")
	fmt.Println("  • Update later: gsc app import git --update")

	// Calculate shadow size
	size, err := ShadowSize(shadowPath)
	if err == nil {
		fmt.Println()
		fmt.Println("Disk Management:")
		fmt.Printf("  • Shadow repo: %s (%s)\n", shadowPath, formatBytes(size))
		fmt.Printf("  • To reclaim space: gsc app import git --delete-shadow --owner %s --repo %s --branch %s\n", owner, repo, branch)
		fmt.Println()
		fmt.Println("  Note: Deleting the shadow repo removes the local snapshot only.")
		fmt.Println("        To remove the imported data from the chat app, use the GitSense Chat UI.")
	}
}

// GetRefChatID returns the ref_chat_id from the complete event
func (p *ProgressUI) GetRefChatID() int64 {
	return p.refChatID
}

// GetDuration returns the total wall-clock duration of the import operation
func (p *ProgressUI) GetDuration() time.Duration {
	return time.Since(p.startTime)
}

// Cleanup ensures the terminal is left in a clean state
func (p *ProgressUI) Cleanup() {
	if p.isTerminal {
		// Clear any remaining shadow progress lines
		if p.shadowNumLines > 0 {
			p.clearShadowLines()
			p.shadowNumLines = 0
		}
		// Clear any remaining multi-line progress
		if p.numLines > 0 {
			for i := 0; i < p.numLines; i++ {
				fmt.Print("\033[F\033[2K")
			}
			p.numLines = 0
		}
		fmt.Println() // Ensure we end on a new line
	}
}

// unmarshalData is a helper to unmarshal json.RawMessage
func unmarshalData(data json.RawMessage, v interface{}) error {
	return json.Unmarshal(data, v)
}

// formatBytes converts bytes to a human-readable string
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
