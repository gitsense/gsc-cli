/**
 * Component: Pi Resume Picker
 * Block-UUID: 7b3f6a2e-9c1d-4e88-bf0a-2a51d4c9e7a1
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Interactive TUI session picker for `gsc pi -r/--resume` and `-q`, handing off to `pi --session`.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: claude-opus-4-8 (v1.0.0)
 */


package pi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// runResumePicker loads sessions from the mirror, shows the picker, and on
// selection replaces the current process with `pi --session <uuid>`.
// query (from -q) pre-filters to sessions whose content matches.
func runResumePicker(dbPath, query string, passthrough []string) error {
	resolvedDB, err := resolvePiSessionsDBPath(dbPath)
	if err != nil {
		return err
	}

	items, err := loadResumeSessions(resolvedDB, query)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		if query != "" {
			fmt.Printf("No Pi sessions match %q.\n", query)
		} else {
			fmt.Println("No Pi sessions found. Run `gsc pi sessions sync` first.")
		}
		return nil
	}

	cwd, _ := os.Getwd()
	model := newPickerModel(items, query, cwd)

	program := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return err
	}

	pm, ok := finalModel.(pickerModel)
	if !ok || pm.chosen == "" {
		return nil // cancelled
	}

	return execPiSession(pm.chosen, pm.chosenDir, passthrough)
}

// resolvePiSessionsDBPath mirrors the resolver in the sessions CLI package.
func resolvePiSessionsDBPath(value string) (string, error) {
	if value != "" {
		return filepath.Abs(value)
	}
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return "", err
	}
	return settings.GetPiSessionsDatabasePath(gscHome), nil
}

// execPiSession changes into the session's working directory and replaces the
// current process with an interactive pi session. Pi organizes sessions by
// working directory, so resuming must happen from where the session ran.
func execPiSession(sessionID, dir string, passthrough []string) error {
	piPath, err := exec.LookPath("pi")
	if err != nil {
		return fmt.Errorf("pi executable not found on PATH: %w", err)
	}
	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			return fmt.Errorf("cannot resume in session directory %s: %w", dir, err)
		}
	}
	argv := append([]string{"pi", "--session", sessionID}, passthrough...)
	// syscall.Exec hands the terminal directly to pi.
	return syscall.Exec(piPath, argv, os.Environ())
}

// sessionItem is one selectable row.
type sessionItem struct {
	id            string
	title         string
	repoRoot      string
	cwd           string
	lastMessageAt string
	messageCount  int
	matchCount    int
}

func loadResumeSessions(dbPath, query string) ([]sessionItem, error) {
	ctx := context.Background()
	const max = 500

	if query != "" {
		results, err := pisessions.QuerySessions(ctx, pisessions.QueryOptions{
			DBPath: dbPath,
			View:   "sessions",
			Text:   query,
			Sort:   "recent",
			Limit:  max,
		})
		if err != nil {
			return nil, err
		}
		items := make([]sessionItem, 0, len(results))
		for _, r := range results {
			items = append(items, sessionItem{
				id:            r.SessionID,
				title:         displayTitle(r.Title),
				repoRoot:      r.RepoRoot,
				cwd:           r.CWD,
				lastMessageAt: r.LastMessageAt,
				messageCount:  r.MessageCount,
				matchCount:    r.MatchedMessageCount,
			})
		}
		return items, nil
	}

	results, err := pisessions.List(ctx, pisessions.ListOptions{
		DBPath: dbPath,
		Sort:   "recent",
		Limit:  max,
	})
	if err != nil {
		return nil, err
	}
	items := make([]sessionItem, 0, len(results))
	for _, r := range results {
		items = append(items, sessionItem{
			id:            r.SessionID,
			title:         displayTitle(r.LastUserText),
			repoRoot:      r.RepoRoot,
			cwd:           r.CWD,
			lastMessageAt: r.LastMessageAt,
			messageCount:  r.MessageCount,
		})
	}
	return items, nil
}

// resumeDir is the directory to launch pi from: the session's working
// directory if known, otherwise its repo root.
func resumeDir(it sessionItem) string {
	if it.cwd != "" {
		return it.cwd
	}
	return it.repoRoot
}

func displayTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(no messages)"
	}
	return s
}

// scopeMode controls which sessions are shown.
type scopeMode int

const (
	scopeAll scopeMode = iota
	scopeCurrent
)

type pickerModel struct {
	all       []sessionItem
	visible   []sessionItem
	cursor    int
	scope     scopeMode
	query     string
	cwd       string
	filter    string // live `/` filter text
	filtering bool
	chosen    string
	chosenDir string
	height    int
}

func newPickerModel(items []sessionItem, query, cwd string) pickerModel {
	m := pickerModel{
		all:    items,
		scope:  scopeAll,
		query:  query,
		cwd:    cwd,
		height: 20,
	}
	m.recompute()
	return m
}

func (m *pickerModel) recompute() {
	var vis []sessionItem
	for _, it := range m.all {
		if m.scope == scopeCurrent && !sameDir(it.cwd, m.cwd) {
			continue
		}
		if m.filter != "" && !matchesFilter(it, m.filter) {
			continue
		}
		vis = append(vis, it)
	}
	m.visible = vis
	if m.cursor >= len(vis) {
		m.cursor = len(vis) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return a == b
}

func matchesFilter(it sessionItem, f string) bool {
	f = strings.ToLower(f)
	return strings.Contains(strings.ToLower(it.title), f) ||
		strings.Contains(strings.ToLower(it.repoRoot), f) ||
		strings.Contains(strings.ToLower(it.cwd), f)
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.filtering {
			return m.updateFiltering(msg)
		}
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
			}
		case "tab":
			if m.scope == scopeAll {
				m.scope = scopeCurrent
			} else {
				m.scope = scopeAll
			}
			m.cursor = 0
			m.recompute()
		case "/":
			m.filtering = true
		case "enter":
			if len(m.visible) > 0 {
				it := m.visible[m.cursor]
				m.chosen = it.id
				m.chosenDir = resumeDir(it)
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m pickerModel) updateFiltering(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filter = ""
		m.recompute()
	case "enter":
		m.filtering = false
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.recompute()
		}
	default:
		if len(msg.String()) == 1 {
			m.filter += msg.String()
			m.recompute()
		}
	}
	return m, nil
}

var (
	styleHeader   = lipgloss.NewStyle().Bold(true)
	styleCursor   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)

func (m pickerModel) View() string {
	var b strings.Builder

	scopeLabel := "all directories"
	if m.scope == scopeCurrent {
		scopeLabel = "current directory"
	}
	header := "Resume a Pi session"
	if m.query != "" {
		header += fmt.Sprintf("  (matching %q)", m.query)
	}
	b.WriteString(styleHeader.Render(header))
	b.WriteString("\n")
	b.WriteString(styleDim.Render(fmt.Sprintf("scope: %s · %d sessions", scopeLabel, len(m.visible))))
	b.WriteString("\n\n")

	if len(m.visible) == 0 {
		b.WriteString(styleDim.Render("  (no sessions in this view)"))
		b.WriteString("\n")
	}

	// Rows fit within available height (2 lines each, reserve room for chrome).
	rowsAvail := (m.height - 7) / 2
	if rowsAvail < 1 {
		rowsAvail = 1
	}
	start := 0
	if m.cursor >= rowsAvail {
		start = m.cursor - rowsAvail + 1
	}
	end := start + rowsAvail
	if end > len(m.visible) {
		end = len(m.visible)
	}

	for i := start; i < end; i++ {
		it := m.visible[i]
		cursor := "  "
		titleStyle := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = styleCursor.Render("> ")
			titleStyle = styleSelected
		}
		idShort := it.id
		if len(idShort) > 8 {
			idShort = idShort[:8]
		}
		line1 := fmt.Sprintf("%s%s  %-12s  %s",
			cursor, idShort, relativeTime(it.lastMessageAt), truncate(it.title, 70))
		b.WriteString(titleStyle.Render(line1))
		b.WriteString("\n")

		loc := locationLabel(it.repoRoot, it.cwd)
		meta := fmt.Sprintf("%d msgs", it.messageCount)
		if it.matchCount > 0 {
			meta += fmt.Sprintf(" · %d matches", it.matchCount)
		}
		b.WriteString(styleDim.Render(fmt.Sprintf("    %s  %s", loc, meta)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.filtering {
		b.WriteString(fmt.Sprintf("/%s", m.filter))
		b.WriteString(styleDim.Render("   (enter to apply · esc to clear)"))
	} else {
		b.WriteString(styleDim.Render("↑/↓ move · enter resume · tab all/current · / filter · q quit"))
	}
	b.WriteString("\n")
	return b.String()
}

func locationLabel(repoRoot, cwd string) string {
	if repoRoot != "" {
		return "~/" + homeRelative(repoRoot)
	}
	if cwd != "" {
		return "~/" + homeRelative(cwd)
	}
	return "(unknown location)"
}

func homeRelative(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return strings.TrimPrefix(path, home)
	}
	return path
}

func relativeTime(ts string) string {
	if ts == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
