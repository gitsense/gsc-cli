/**
 * Component: Native App CLI Logs
 * Block-UUID: 112bb64d-fa78-4810-aaf7-46cd767b7e9b
 * Parent-UUID: 981f8ba7-ffd1-41a1-afd8-f1882f232ec2
 * Version: 1.5.0
 * Description: Implements the 'gsc app native logs' command to view or tail the application logs, with support for listing available log files, showing last N lines, and following logs in real-time. Updated to use FormatUptime from internal/app package to avoid import cycle. Enhanced --follow to behave like tail -f with polling, file rotation detection, and graceful signal handling.
 * Language: Go
 * Created-at: 2026-05-30T17:46:15.046Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
 */


package native

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/native"
	"github.com/gitsense/gsc-cli/pkg/settings"
	app_internal "github.com/gitsense/gsc-cli/internal/app"
)

var (
	logsDataDir string
	logsFollow  bool
	logsLines   int
	logsList    bool
)

// logsCmd represents the native app logs command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View or tail the GitSense Chat application logs",
	Long: `Displays the application logs. By default, shows the last 100 lines.
Use --follow to tail the logs in real-time.
Use --list to show all available log files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve Data Directory
		dataDir := logsDataDir
		if dataDir == "" {
			// Priority 1: GSC_HOME/data (if GSC_HOME is set)
			if gscHomeEnv := os.Getenv("GSC_HOME"); gscHomeEnv != "" {
				dataDir = filepath.Join(gscHomeEnv, "data")
			} else {
				// Priority 2: native-config.json
				gscHome, err := settings.GetGSCHome(false)
				if err != nil {
					return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
				}
				cfg, err := native.LoadConfig(gscHome)
				if err != nil {
					return fmt.Errorf("failed to load native config: %w", err)
				}
				if cfg != nil && cfg.DataDir != "" {
					dataDir = cfg.DataDir
				} else {
					// Priority 3: Default
					dataDir = filepath.Join(gscHome, settings.AppDataDirRelPath)
				}
			}
		}
		absDataDir, err := filepath.Abs(dataDir)
		if err != nil {
			return fmt.Errorf("failed to resolve data directory: %w", err)
		}
		dataDir = absDataDir

		// 2. Resolve log directory
		logDir := filepath.Join(dataDir, "logs")
		if _, err := os.Stat(logDir); os.IsNotExist(err) {
			return fmt.Errorf("log directory not found: %s\nHas the application been started?", logDir)
		}

		// 3. List log files if requested
		if logsList {
			return listLogFiles(logDir)
		}

		// 4. Resolve log file path
		logFile := filepath.Join(logDir, "app.log")
		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			return fmt.Errorf("log file not found: %s\nHas the application been started?", logFile)
		}

		// 5. Display logs
		if logsFollow {
			// Tail logs in real-time
			return tailLogs(logFile)
		} else {
			// Show last N lines
			return showLastLines(logFile, logsLines)
		}
	},
}

func init() {
	NativeCmd.AddCommand(logsCmd)

	logsCmd.Flags().StringVar(&logsDataDir, "data-dir", "", "Override the data directory (default: from GSC_HOME/data or native-config.json)")
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output in real-time")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 100, "Number of lines to show (default: 100)")
	logsCmd.Flags().BoolVar(&logsList, "list", false, "List all available log files")
}

// listLogFiles lists all available log files in the log directory
func listLogFiles(logDir string) error {
	fmt.Println("\n" + "━" + strings.Repeat("━", 58))
	fmt.Println("  Available Log Files")
	fmt.Println("━" + strings.Repeat("━", 58))

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("  No log files found")
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		size := info.Size()
		var sizeStr string
		if size < 1024 {
			sizeStr = fmt.Sprintf("%d B", size)
		} else if size < 1024*1024 {
			sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
		} else {
			sizeStr = fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
		}

		modTime := info.ModTime()
		timeAgo := time.Since(modTime)
		timeAgoStr := app_internal.FormatUptime(timeAgo)

		isCurrent := entry.Name() == "app.log"
		currentMarker := ""
		if isCurrent {
			currentMarker = " (current)"
		}

		fmt.Printf("  %s%s\n", entry.Name(), currentMarker)
		fmt.Printf("    Size: %s, Modified: %s (%s ago)\n", sizeStr, modTime.Format("2006-01-02 15:04:05"), timeAgoStr)
	}

	fmt.Println("━" + strings.Repeat("━", 58))
	return nil
}

// showLastLines displays the last N lines from the log file
func showLastLines(logFile string, lines int) error {
	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Read all lines
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	// Get last N lines
	start := 0
	if len(allLines) > lines {
		start = len(allLines) - lines
	}

	// Display lines
	for i := start; i < len(allLines); i++ {
		fmt.Println(allLines[i])
	}

	return nil
}

// CircularBuffer efficiently stores the last N lines
type CircularBuffer struct {
	buffer []string
	size   int
	index  int
	count  int
}

// NewCircularBuffer creates a new circular buffer with the specified size
func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{
		buffer: make([]string, size),
		size:   size,
		index:  0,
		count:  0,
	}
}

// Add adds a line to the buffer
func (cb *CircularBuffer) Add(line string) {
	cb.buffer[cb.index] = line
	cb.index = (cb.index + 1) % cb.size
	if cb.count < cb.size {
		cb.count++
	}
}

// GetLines returns all lines in the buffer in order
func (cb *CircularBuffer) GetLines() []string {
	if cb.count == 0 {
		return []string{}
	}

	result := make([]string, cb.count)
	start := cb.index - cb.count
	if start < 0 {
		start += cb.size
	}

	for i := 0; i < cb.count; i++ {
		result[i] = cb.buffer[(start+i)%cb.size]
	}

	return result
}

// tailLogs follows the log file in real-time, behaving like tail -f
func tailLogs(logFile string) error {
	// Set up signal handling for graceful exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Open the log file
	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Step 1: Read and display the last N lines using circular buffer
	cb := NewCircularBuffer(logsLines)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		cb.Add(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	// Display the buffered lines
	for _, line := range cb.GetLines() {
		fmt.Println(line)
	}

	// Step 2: Seek to end and follow with polling
	_, err = file.Seek(0, 2)
	if err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}

	// Create reader for polling
	reader := bufio.NewReader(file)

	// Track file metadata for rotation detection
	var lastSize int64
	var lastModTime time.Time
	info, err := file.Stat()
	if err == nil {
		lastSize = info.Size()
		lastModTime = info.ModTime()
	}

	// Polling loop with error retry
	const maxRetries = 5
	retryCount := 0
	pollInterval := 100 * time.Millisecond

	for {
		select {
		case <-sigChan:
			// Graceful exit on interrupt
			fmt.Println("\nStopping log follow...")
			return nil
		default:
			// Continue polling
		}

		// Check for file rotation
		info, err := file.Stat()
		if err != nil {
			// File error - retry with exponential backoff
			retryCount++
			if retryCount >= maxRetries {
				return fmt.Errorf("failed to stat log file after %d retries: %w", maxRetries, err)
			}
			time.Sleep(time.Duration(retryCount) * time.Second)
			continue
		}

		// Detect rotation: size decreased or mod time went backward
		if info.Size() < lastSize || info.ModTime().Before(lastModTime) {
			// File was rotated - close and reopen
			file.Close()
			
			file, err = os.Open(logFile)
			if err != nil {
				return fmt.Errorf("failed to reopen log file after rotation: %w", err)
			}
			defer file.Close()
			
			reader = bufio.NewReader(file)
			lastSize = 0
			lastModTime = info.ModTime()
			retryCount = 0
			continue
		}

		// Update metadata
		lastSize = info.Size()
		lastModTime = info.ModTime()

		// Try to read a line
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// No new data - sleep and retry
				time.Sleep(pollInterval)
				retryCount = 0
				continue
			}
			// Real error - retry with exponential backoff
			retryCount++
			if retryCount >= maxRetries {
				return fmt.Errorf("failed to read log file after %d retries: %w", maxRetries, err)
			}
			time.Sleep(time.Duration(retryCount) * time.Second)
			continue
		}

		// Successfully read a line - print it
		fmt.Print(line)
		retryCount = 0
	}
}
