/**
 * Component: Search Response Formatter
 * Block-UUID: dc14101c-f555-436c-9085-ce180beb054a
 * Parent-UUID: 7ad6b101-489a-4539-8329-41abc2940bde
 * Version: 2.8.0
 * Description: Added Chat ID display to human-readable output for analyzed files and introduced ShowChatID option for configurability.
 * Language: Go
 * Created-at: 2026-02-06T03:19:11.985Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), Gemini 3 Flash (v2.2.0), Gemini 3 Flash (v2.3.0), Gemini 3 Flash (v2.3.1), GLM-4.7 (v2.4.0), Gemini 3 Flash (v2.5.0), Gemini 3 Flash (v2.6.0), Gemini 3 Flash (v2.7.0), Gemini 3 Flash (v2.8.0)
 */


package search

import (
	"encoding/json"
	"fmt"
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

	return formatHumanResponse(summary, matches, opts)
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

func formatHumanResponse(summary GrepSummary, matches []MatchResult, opts FormatOptions) error {
	files := GroupMatchesByFile(matches)
	useColor := output.IsTerminal()

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
			for k, v := range file.Metadata {
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
					key = logger.ColorYellow + k + logger.ColorReset
				}
				fmt.Printf("; %s: %v\n", key, v)
				metadataPrinted = true
			}
		}

		// Blank line after metadata section if it was printed
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

	return nil
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
