/**
 * Component: Search Response Formatter
 * Block-UUID: 7a05736c-8017-4921-b7a3-826ab2b37b8a
 * Parent-UUID: 24e8fee6-f595-4ef0-9c44-94ecc9c31728
 * Version: 2.10.0
 * Description: Added Chat ID display to human-readable output for analyzed files and introduced ShowChatID option for configurability.
 * Language: Go
 * Created-at: 2026-02-06T04:51:36.025Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), Gemini 3 Flash (v2.2.0), Gemini 3 Flash (v2.3.0), Gemini 3 Flash (v2.3.1), GLM-4.7 (v2.4.0), Gemini 3 Flash (v2.5.0), Gemini 3 Flash (v2.6.0), Gemini 3 Flash (v2.7.0), Gemini 3 Flash (v2.8.0), Gemini 3 Flash (v2.9.0), Gemini 3 Flash (v2.10.0)
 */


package search

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yourusername/gsc-cli/internal/output"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// FormatOptions holds configuration for the output formatter.
type FormatOptions struct {
	Format          string
	SummaryOnly     bool
	NoFields        bool
	ShowChatID      bool
	RequestedFields []string
	Filters         []string
	AvailableFields []string
}

// FormatResponse constructs the final JSON response and prints it to stdout.
func FormatResponse(context QueryContext, summary GrepSummary, matches []MatchResult, opts FormatOptions) error {
	// Populate filters in the context to ensure they appear in the JSON output
	context.Filters = opts.Filters
	context.RequestedFields = opts.RequestedFields
	context.AvailableFields = opts.AvailableFields

	if opts.Format == "json" {
		return formatJSONResponse(context, summary, matches, opts.SummaryOnly)
	}

	return formatHumanResponse(context, summary, matches, opts)
}

func formatJSONResponse(context QueryContext, summary GrepSummary, matches []MatchResult, summaryOnly bool) error {
	response := GrepResponse{
		Context: context,
		Summary: summary,
	}

	if !summaryOnly {
		response.Files = GroupMatchesByFile(matches)
	}

	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON response: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func formatHumanResponse(context QueryContext, summary GrepSummary, matches []MatchResult, opts FormatOptions) error {
	files := GroupMatchesByFile(matches)
	useColor := output.IsTerminal()

	printIntelligenceHeader(context, summary, useColor)

	// File Card Layout
	for _, file := range files {
		status := "  "
		if file.Analyzed {
			status = "✓ "
			if useColor {
				status = logger.ColorGreen + status + logger.ColorReset
			}
		} else {
			status = "x "
			if useColor {
				status = logger.ColorRed + status + logger.ColorReset
			}
		}

		// Colorize filename
		filePath := file.FilePath
		if useColor {
			filePath = logger.ColorBold + logger.ColorCyan + filePath + logger.ColorReset
		}

		// Append Chat ID if requested and available
		chatIDStr := ""
		if opts.ShowChatID && file.Analyzed && file.ChatID != nil {
			chatIDStr = fmt.Sprintf(" (chat-id: %v)", *file.ChatID)
			if useColor {
				// Use a subtle white/gray for the chat ID to keep focus on the path
				chatIDStr = logger.ColorWhite + chatIDStr + logger.ColorReset
			}
		}

		fmt.Printf("%s%s%s\n", status, filePath, chatIDStr)

		// Show metadata if not disabled
		metadataPrinted := false
		if !opts.NoFields && file.Analyzed && len(file.Metadata) > 0 {
			// Get and sort keys for consistent output
			var keys []string
			for k := range file.Metadata {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				// Respect RequestedFields if provided
				if len(opts.RequestedFields) > 0 {
					found := false
					for _, rf := range opts.RequestedFields {
						if rf == k {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}

				key := k
				if useColor {
					key = logger.ColorYellow + key + logger.ColorReset
				}
				fmt.Printf("; %s: %v\n", key, file.Metadata[k])
				metadataPrinted = true
			}
		}

		if metadataPrinted {
			fmt.Println()
		}

		if opts.SummaryOnly {
			matchCount := 0
			for _, m := range matches {
				if m.FilePath == file.FilePath {
					matchCount++
				}
			}
			fmt.Printf("  ; matches: %d\n", matchCount)
		} else {
			for _, m := range file.Matches {
				lineNum := fmt.Sprintf("%d", m.LineNumber)
				// Trim trailing newlines from the source line to prevent double spacing
				lineText := strings.TrimRight(m.LineText, "\r\n")

				if useColor {
					// Use Red for line numbers to distinguish from status checkmarks
					lineNum = logger.ColorRed + lineNum + logger.ColorReset
					lineText = highlightText(lineText, m.Submatches)
				}

				fmt.Printf("%s:%s\n", lineNum, lineText)
			}
		}
		fmt.Print("\n\n") // Two blank lines after matches (file separator)
	}

	if summary.MatchesOutsideScope > 0 {
		printHintFooter(summary.MatchesOutsideScope, useColor)
	}

	return nil
}

func printIntelligenceHeader(ctx QueryContext, summary GrepSummary, useColor bool) {
	divider := "# ──────────────────────────────────────────────────────────────────────────────"
	if useColor {
		divider = logger.ColorWhite + divider + logger.ColorReset
	}

	fmt.Println(divider)
	if ctx.ProfileName != "" {
		profile := ctx.ProfileName
		if useColor { profile = logger.ColorBold + profile + logger.ColorReset }
		fmt.Printf("# Context:  %s (Active Profile)\n", profile)
	}

	fmt.Printf("# Search:   %s\n", ctx.Pattern)
	
	brain := ctx.Database
	if useColor { brain = logger.ColorCyan + brain + logger.ColorReset }
	fmt.Printf("# Database:    %s\n", brain)

	if ctx.ScopeSummary != "" {
		fmt.Printf("# Scope:    %s\n", ctx.ScopeSummary)
	}

	coverage := 0
	if summary.TotalFiles > 0 {
		coverage = (summary.AnalyzedFiles * 100) / summary.TotalFiles
	}
	
	fmt.Printf("# Summary:  %d matches in %d files (%d%% analyzed coverage)\n", 
		summary.TotalMatches, summary.TotalFiles, coverage)
	fmt.Println(divider)
	fmt.Println()
}

func printHintFooter(outsideCount int, useColor bool) {
	divider := "# ──────────────────────────────────────────────────────────────────────────────"
	hint := "Hint:"
	if useColor {
		divider = logger.ColorWhite + divider + logger.ColorReset
		hint = logger.ColorYellow + hint + logger.ColorReset
	}

	fmt.Println(divider)
	fmt.Printf("# %s %d matches found outside of current Focus Scope. \n", hint, outsideCount)
	fmt.Println("# Run 'gsc config scope clear' to see all results.")
	fmt.Println(divider)
}

func highlightText(text string, offsets []MatchOffset) string {
	if len(offsets) == 0 {
		return text
	}

	var sb strings.Builder
	lastEnd := 0

	for _, offset := range offsets {
		// Append text before the match
		sb.WriteString(text[lastEnd:offset.Start])
		// Append highlighted match using Bold Purple for high visibility and uniqueness
		sb.WriteString(logger.ColorBold + logger.ColorPurple)
		sb.WriteString(text[offset.Start:offset.End])
		sb.WriteString(logger.ColorReset)
		lastEnd = offset.End
	}

	// Append remaining text
	sb.WriteString(text[lastEnd:])
	return sb.String()
}

// GroupMatchesByFile converts a flat list of matches into a list of file results.
func GroupMatchesByFile(matches []MatchResult) []FileResult {
	fileMap := make(map[string]*FileResult)

	for _, m := range matches {
		if _, exists := fileMap[m.FilePath]; !exists {
			fr := FileResult{
				FilePath: m.FilePath,
				Analyzed: len(m.Metadata) > 0,
				Matches:  []MatchDetail{},
			}
			if fr.Analyzed {
				id := m.ChatID
				fr.ChatID = &id
				fr.Metadata = m.Metadata
			}
			fileMap[m.FilePath] = &fr
		}

		// Append match detail
		fileMap[m.FilePath].Matches = append(fileMap[m.FilePath].Matches, MatchDetail{
			LineNumber:    m.LineNumber,
			LineText:      m.LineText,
			Submatches:    m.Submatches,
			ContextBefore: m.ContextBefore,
			ContextAfter:  m.ContextAfter,
		})
	}

	// Convert map to slice
	var results []FileResult
	for _, fr := range fileMap {
		results = append(results, *fr)
	}

	return results
}
