/**
 * Component: Pi Resume Picker
 * Block-UUID: c4d9e1f6-8a32-4b07-9d15-6f2e3a8c0b91
 * Parent-UUID: 7973de21-0dd6-4a15-97ad-34d203555211
 * Version: 2.6.0
 * Description: Split-pane interactive TUI for `gsc pi -r`: tab-cycled focus zones (list/preview/options), always-on search, first/last-N message preview with smart truncation (first/last 5 lines for long messages), and a cyclable row density, handing off to `pi --session`. Adds an orange focus-accent border per pane, a brightness-based key/value hierarchy, a strong footer rule, left-padded preview, a widened list showing each session's opening line, and an orange-bracket indicator for the selected options control. Extracts pickSession so other commands (e.g. `gsc pi --hud`) can reuse the picker and supply their own handoff. Now renders markdown formatting in the preview pane with code blocks, bold text, inline code, and lists. Messages are displayed with role-specific colored labels and full-width dividers for visual separation.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: claude-opus-4-8 (v1.0.0, v2.0.0, v2.1.0, v2.2.0), MiMo-v2.5-Pro (v2.3.0, v2.4.0)
 */


package pi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

const (
	maxSessions  = 500
	previewN     = 5
	previewLines = 5
)

// runResumePicker shows the picker and, on selection, replaces the current
// process with `pi --session <uuid>`.
func runResumePicker(dbPath string, passthrough []string) error {
	id, dir, err := pickSession(dbPath)
	if err != nil || id == "" {
		return err
	}
	return execPiSession(id, dir, passthrough)
}

// pickSession loads sessions from the mirror and runs the split-pane picker,
// returning the chosen session UUID and the directory to launch it from. Both
// are empty when the user cancels. Callers supply their own handoff (resume
// into pi, launch a tmux HUD, etc.), so the picker UI stays shared.
func pickSession(dbPath string) (id, dir string, err error) {
	resolvedDB, err := resolvePiSessionsDBPath(dbPath)
	if err != nil {
		return "", "", err
	}

	items, err := loadResumeSessions(resolvedDB)
	if err != nil {
		return "", "", err
	}
	if len(items) == 0 {
		fmt.Println("No Pi sessions found. Run `gsc pi sessions sync` first.")
		return "", "", nil
	}

	cwd, _ := os.Getwd()
	model := newPickerModel(resolvedDB, items, cwd)

	finalModel, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		return "", "", err
	}

	pm, ok := finalModel.(pickerModel)
	if !ok || pm.chosen == "" {
		return "", "", nil // cancelled
	}
	return pm.chosen, pm.chosenDir, nil
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
	createdAt     string
	lastMessageAt string
	messageCount  int
}

func loadResumeSessions(dbPath string) ([]sessionItem, error) {
	results, err := pisessions.List(context.Background(), pisessions.ListOptions{
		DBPath: dbPath,
		Sort:   "recent",
		Limit:  maxSessions,
	})
	if err != nil {
		return nil, err
	}
	items := make([]sessionItem, 0, len(results))
	for _, r := range results {
		items = append(items, sessionItem{
			id:            r.SessionID,
			title:         rowTitle(r.Name, r.FirstUserText),
			repoRoot:      r.RepoRoot,
			cwd:           r.CWD,
			createdAt:     r.CreatedAt,
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

// rowTitle is the left-pane label: the session's name if it has one, otherwise
// its first user message. The preview pane shows recent activity, so leading
// with the opening line gives both ends of the session at a glance.
func rowTitle(name, firstUserText string) string {
	if s := collapseWS(name); s != "" {
		return s
	}
	if s := collapseWS(firstUserText); s != "" {
		return s
	}
	return "(no messages)"
}

// --- modes -----------------------------------------------------------------

type focusZone int

const (
	zoneList focusZone = iota
	zonePreview
	zoneOptions
)

type scopeMode int

const (
	scopeAll scopeMode = iota
	scopeCurrent
)

type sortMode int

const (
	sortUpdated sortMode = iota
	sortCreated
)

type rangeMode int

const (
	rangeLast rangeMode = iota
	rangeFirst
)

// optControl indexes the segmented controls in the Options zone.
const (
	ctlScope = iota
	ctlSort
	ctlRange
	ctlCount
)

// density controls how many lines each list row occupies. Implemented as a
// cycle so a future verbose mode is just another entry.
type density int

const (
	densityComfortable density = iota
	densityCompact
	densityCount
)

func (d density) rowHeight() int {
	if d == densityCompact {
		return 1
	}
	return 2
}

// --- model -----------------------------------------------------------------

type pickerModel struct {
	dbPath  string
	all     []sessionItem
	visible []sessionItem
	cwd     string

	focus      focusZone
	optControl int
	scope      scopeMode
	sort       sortMode
	rng        rangeMode
	density    density

	cursor        int
	search        string
	preview       []pisessions.MessagePreview
	previewKey    string
	previewScroll int

	chosen    string
	chosenDir string

	width  int
	height int
}

func newPickerModel(dbPath string, items []sessionItem, cwd string) pickerModel {
	m := pickerModel{
		dbPath:  dbPath,
		all:     items,
		cwd:     cwd,
		focus:   zoneList,
		scope:   scopeAll,
		sort:    sortUpdated,
		rng:     rangeLast,
		density: densityComfortable,
		width:   80,
		height:  24,
	}
	m.recompute()
	return m
}

func (m pickerModel) Init() tea.Cmd {
	// Trigger an initial tick so Update can set previewKey and fire the cmd.
	return func() tea.Msg { return initTickMsg{} }
}

type initTickMsg struct{}

// recompute rebuilds the visible slice from scope, search, and sort.
func (m *pickerModel) recompute() {
	var vis []sessionItem
	for _, it := range m.all {
		if m.scope == scopeCurrent && !sameDir(it.cwd, m.cwd) {
			continue
		}
		if m.search != "" && !matchesFilter(it, m.search) {
			continue
		}
		vis = append(vis, it)
	}
	sort.SliceStable(vis, func(i, j int) bool {
		if m.sort == sortCreated {
			return vis[i].createdAt > vis[j].createdAt
		}
		return vis[i].lastMessageAt > vis[j].lastMessageAt
	})
	m.visible = vis
	if m.cursor >= len(vis) {
		m.cursor = len(vis) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m pickerModel) currentItem() (sessionItem, bool) {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return sessionItem{}, false
	}
	return m.visible[m.cursor], true
}

func previewKeyFor(id string, fromEnd bool) string {
	if fromEnd {
		return id + "#last"
	}
	return id + "#first"
}

// currentPreviewCmd fetches the preview for the highlighted session, unless it
// is already loaded. Returns nil when there is nothing to load.
func (m *pickerModel) currentPreviewCmd() tea.Cmd {
	it, ok := m.currentItem()
	if !ok {
		m.preview = nil
		m.previewKey = ""
		return nil
	}
	fromEnd := m.rng == rangeLast
	key := previewKeyFor(it.id, fromEnd)
	if key == m.previewKey {
		return nil // already loaded/loading for this selection
	}
	m.preview = nil
	m.previewScroll = 0
	m.previewKey = key
	dbPath := m.dbPath
	id := it.id
	return func() tea.Msg {
		msgs, err := pisessions.Messages(context.Background(), pisessions.PreviewOptions{
			DBPath:    dbPath,
			SessionID: id,
			Limit:     previewN,
			FromEnd:   fromEnd,
		})
		return previewMsg{key: key, msgs: msgs, err: err}
	}
}

type previewMsg struct {
	key  string
	msgs []pisessions.MessagePreview
	err  error
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

// --- update ----------------------------------------------------------------

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case initTickMsg:
		return m, m.currentPreviewCmd()

	case previewMsg:
		if msg.key == m.previewKey {
			m.preview = msg.msgs
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m pickerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "tab":
		m.focus = (m.focus + 1) % 3
		return m, nil
	case "shift+tab":
		m.focus = (m.focus + 2) % 3
		return m, nil
	case "ctrl+o":
		m.density = (m.density + 1) % densityCount
		return m, nil
	case "enter":
		return m.handleEnter()
	case "up":
		return m.handleVertical(-1)
	case "down":
		return m.handleVertical(1)
	case "left":
		if m.focus == zoneOptions {
			m.optControl = (m.optControl + ctlCount - 1) % ctlCount
		}
		return m, nil
	case "right":
		if m.focus == zoneOptions {
			m.optControl = (m.optControl + 1) % ctlCount
		}
		return m, nil
	case "esc":
		if m.search != "" {
			m.search = ""
			m.recompute()
			return m, m.currentPreviewCmd()
		}
		return m, tea.Quit
	case "backspace":
		if m.search != "" {
			m.search = m.search[:len(m.search)-1]
			m.recompute()
			return m, m.currentPreviewCmd()
		}
		return m, nil
	case "space":
		m.search += " "
		m.recompute()
		return m, m.currentPreviewCmd()
	default:
		// Always-on search: printable runes feed the search box regardless of
		// the focused zone (no zone binds letter keys).
		if msg.Type == tea.KeyRunes {
			m.search += string(msg.Runes)
			m.recompute()
			return m, m.currentPreviewCmd()
		}
	}
	return m, nil
}

func (m pickerModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.focus {
	case zoneList:
		if it, ok := m.currentItem(); ok {
			m.chosen = it.id
			m.chosenDir = resumeDir(it)
		}
		return m, tea.Quit
	case zoneOptions:
		return m.changeControl()
	}
	return m, nil
}

// handleVertical applies up/down within the focused zone.
func (m pickerModel) handleVertical(delta int) (tea.Model, tea.Cmd) {
	switch m.focus {
	case zoneList:
		m.cursor += delta
		if m.cursor < 0 {
			m.cursor = 0
		}
		if m.cursor > len(m.visible)-1 {
			m.cursor = len(m.visible) - 1
		}
		return m, m.currentPreviewCmd()
	case zonePreview:
		m.previewScroll += delta
		if m.previewScroll < 0 {
			m.previewScroll = 0
		}
		return m, nil
	case zoneOptions:
		return m.changeControl()
	}
	return m, nil
}

// changeControl cycles the value of the focused Options control. All controls
// are binary today, so direction does not matter.
func (m pickerModel) changeControl() (tea.Model, tea.Cmd) {
	switch m.optControl {
	case ctlScope:
		m.scope = (m.scope + 1) % 2
		m.recompute()
		return m, m.currentPreviewCmd()
	case ctlSort:
		m.sort = (m.sort + 1) % 2
		m.recompute()
		return m, m.currentPreviewCmd()
	case ctlRange:
		m.rng = (m.rng + 1) % 2
		return m, m.currentPreviewCmd()
	}
	return m, nil
}

// --- view ------------------------------------------------------------------

// colorAccent (orange) is the single signal for whatever currently has focus:
// the focused pane's top border and the focused options labels both use it.
const (
	colorAccent = lipgloss.Color("208")
	colorMuted  = lipgloss.Color("238")
)

var (
	styleCursor  = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleSel     = lipgloss.NewStyle().Foreground(colorAccent)
	styleAccent     = lipgloss.NewStyle().Foreground(colorAccent)
	styleAccentBold = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleTag     = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	styleDivider = lipgloss.NewStyle().Foreground(colorMuted)
	styleRule    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleEllipsis = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Faint(true)

	// Role-specific styles for message headers
	styleUser      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))  // Blue
	styleAssistant = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208")) // Orange
	styleTool      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244")) // Gray
	styleBash      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))   // Green
	styleSystem    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))   // Magenta
	styleMessage   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")) // White
	styleRoleDivider = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))         // Subtle gray

	// Brightness language: bright keys/active values draw the eye to what is
	// actionable or selected; dim labels carry the meaning.
	styleKey   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")) // bright = the key to press
	styleValue = lipgloss.NewStyle().Foreground(lipgloss.Color("231"))            // bright = active selection
)

// hint renders a footer key hint: bright key, dim description.
func hint(key, label string) string {
	return styleKey.Render(key) + " " + styleDim.Render(label)
}

// previewPadLeft is the left breathing room inside the preview pane.
const previewPadLeft = 2

// topPad is the number of blank rows above the options bar.
const topPad = 0

// paneStyle wraps a pane's content with a top border that lights orange when
// the pane has focus and is muted otherwise. padLeft adds interior left padding;
// since Width includes padding, the content area shrinks accordingly.
func paneStyle(width int, focused bool, padLeft int) lipgloss.Style {
	border := colorMuted
	if focused {
		border = colorAccent
	}
	return lipgloss.NewStyle().
		Width(width).
		PaddingLeft(padLeft).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(border)
}

func (m pickerModel) View() string {
	leftWidth := m.width*2/5 + 10
	if leftWidth < 40 {
		leftWidth = 40
	}
	if leftWidth > 60 {
		leftWidth = 60
	}
	rightWidth := m.width - leftWidth - 1
	if rightWidth < 20 {
		rightWidth = 20
	}

	totalWidth := leftWidth + 1 + rightWidth

	// Chrome is 4 lines (options bar, the pane top-border row, the footer rule,
	// and the footer text) plus topPad blank rows above the options bar. The
	// body fills the rest so the footer stays pinned to the bottom edge.
	bodyHeight := m.height - 4 - topPad
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	left := padLines(m.renderList(leftWidth, bodyHeight), bodyHeight)
	right := padLines(m.renderPreview(rightWidth-previewPadLeft, bodyHeight), bodyHeight)

	leftBlock := paneStyle(leftWidth, m.focus == zoneList, 0).Render(strings.Join(left, "\n"))
	rightBlock := paneStyle(rightWidth, m.focus == zonePreview, previewPadLeft).Render(strings.Join(right, "\n"))
	divider := renderDivider(bodyHeight)

	var b strings.Builder
	b.WriteString(strings.Repeat("\n", topPad))
	b.WriteString(m.renderTopBar(totalWidth))
	b.WriteString("\n")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, divider, rightBlock))
	b.WriteString("\n")
	b.WriteString(styleRule.Render(strings.Repeat("━", totalWidth)))
	b.WriteString("\n")
	b.WriteString(m.renderFooter())
	return b.String()
}

func (m pickerModel) renderTopBar(width int) string {
	searchLabel := "Type to search"
	if m.search != "" {
		searchLabel = "Search: " + m.search
	}
	left := styleDim.Render(searchLabel)
	controls := m.renderControls()
	gap := width - lipgloss.Width(left) - lipgloss.Width(controls)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + controls
}

func (m pickerModel) renderControls() string {
	z := m.focus == zoneOptions
	scope := segment("Scope", []string{"All", "Cwd"}, int(m.scope), z, m.optControl == ctlScope)
	srt := segment("Sort", []string{"Updated", "Created"}, int(m.sort), z, m.optControl == ctlSort)
	rng := segment("Range", []string{"Last", "First"}, int(m.rng), z, m.optControl == ctlRange)
	return scope + "   " + srt + "   " + rng
}

// segment renders one labeled control. When the Options zone is focused every
// label turns orange (so it's obvious focus is there). The control that ←/→
// landed on renders its active value in orange brackets (vs white for the
// others), so it's clear which control ↑/↓ will change.
func segment(label string, values []string, active int, zoneFocused, selected bool) string {
	labelStyle := styleDim
	if zoneFocused {
		labelStyle = styleAccent
	}
	activeStyle := styleValue
	if zoneFocused && selected {
		labelStyle = labelStyle.Bold(true)
		activeStyle = styleAccentBold
	}
	parts := make([]string, len(values))
	for i, v := range values {
		if i == active {
			parts[i] = activeStyle.Render("[" + v + "]")
		} else {
			parts[i] = styleDim.Render(v)
		}
	}
	return labelStyle.Render(label+":") + " " + strings.Join(parts, " ")
}

func (m pickerModel) renderList(width, height int) []string {
	if len(m.visible) == 0 {
		return []string{styleDim.Render("(no sessions in this view)")}
	}
	rh := m.density.rowHeight()
	rowsAvail := height / rh
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

	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, m.renderRow(m.visible[i], i == m.cursor, width)...)
	}
	return lines
}

// renderRow returns the line(s) for one session row: one line in compact, two
// in comfortable. The windowing math derives row count from density.rowHeight,
// so adding a richer mode needs no changes here beyond a new branch.
func (m pickerModel) renderRow(it sessionItem, selected bool, width int) []string {
	marker := "  "
	titleStyle := lipgloss.NewStyle()
	if selected {
		marker = styleCursor.Render("❯ ")
		titleStyle = styleSel
	}
	when := relativeTime(it.lastMessageAt)
	if m.sort == sortCreated {
		when = relativeTime(it.createdAt)
	}

	if m.density == densityCompact {
		meta := fmt.Sprintf(" · %d msgs", it.messageCount)
		avail := width - 2 - 10 - len(meta)
		line := fmt.Sprintf("%s%-9s %s%s", marker, when, truncate(it.title, avail), styleDim.Render(meta))
		return []string{line}
	}

	line1 := fmt.Sprintf("%s%-9s %s", marker, when, truncate(it.title, width-12))
	loc := locationLabel(it.repoRoot, it.cwd)
	meta := fmt.Sprintf("%s · %d msgs", truncate(loc, width-16), it.messageCount)
	return []string{titleStyle.Render(line1), styleDim.Render("    " + meta)}
}

func (m pickerModel) renderPreview(width, height int) []string {
	rangeLabel := "last"
	if m.rng == rangeFirst {
		rangeLabel = "first"
	}
	head := styleDim.Render(fmt.Sprintf("── %s %d messages ──", rangeLabel, previewN))

	if _, ok := m.currentItem(); !ok {
		return []string{head}
	}
	if m.preview == nil {
		return []string{head, "", styleDim.Render("  loading…")}
	}
	if len(m.preview) == 0 {
		return []string{head, "", styleDim.Render("  (no messages)")}
	}

	var lines []string
	lines = append(lines, head, "")
	for i, msg := range m.preview {
		// Add extra blank line between messages (not before the first)
		if i > 0 {
			lines = append(lines, "")
		}
		// Render message header with styled role label
		tag := messageTagStyled(msg)
		lines = append(lines, tag)
		// Add full-width divider line
		lines = append(lines, styleRoleDivider.Render(strings.Repeat("─", width)))
		// Render message content without indentation
		truncated := truncateMessageLines(msg.Text, previewLines)
		rendered := renderMarkdown(truncated, width)
		lines = append(lines, rendered...)
		lines = append(lines, "")
	}

	// Apply scroll, keeping the header pinned.
	body := lines[1:]
	scroll := m.previewScroll
	if scroll >= len(body) {
		scroll = len(body) - 1
	}
	if scroll > 0 {
		body = body[scroll:]
	}
	out := append([]string{head}, body...)
	if len(out) > height {
		out = out[:height]
	}
	return out
}

func messageTag(msg pisessions.MessagePreview) string {
	// Map roles/types to nicely formatted uppercase labels
	label := ""
	if msg.Role != "" {
		switch msg.Role {
		case "user":
			label = "USER"
		case "assistant":
			label = "ASSISTANT"
		case "toolResult":
			label = "TOOL RESULT"
		case "toolCall":
			label = "TOOL CALL"
		default:
			label = strings.ToUpper(msg.Role)
		}
	} else if msg.Type != "" {
		switch msg.Type {
		case "bashExecution":
			label = "BASH"
		case "model_change":
			label = "MODEL CHANGE"
		case "thinking_level_change":
			label = "THINKING LEVEL"
		case "compaction":
			label = "COMPACTION"
		case "session_info":
			label = "SESSION INFO"
		case "custom_message":
			label = "CUSTOM"
		default:
			label = strings.ToUpper(strings.ReplaceAll(msg.Type, "_", " "))
		}
	} else {
		label = "MESSAGE"
	}
	return label
}

// messageTagStyled returns the styled label for a message role/type.
func messageTagStyled(msg pisessions.MessagePreview) string {
	label := messageTag(msg)
	
	// Choose style based on role/type
	var style lipgloss.Style
	if msg.Role != "" {
		switch msg.Role {
		case "user":
			style = styleUser
		case "assistant":
			style = styleAssistant
		case "toolResult", "toolCall":
			style = styleTool
		default:
			style = styleMessage
		}
	} else if msg.Type != "" {
		switch msg.Type {
		case "bashExecution":
			style = styleBash
		case "model_change", "thinking_level_change":
			style = styleSystem
		case "compaction":
			style = styleDim
		default:
			style = styleMessage
		}
	} else {
		style = styleMessage
	}
	
	return style.Render(label)
}

func (m pickerModel) renderFooter() string {
	focus := []string{"List", "Preview", "Options"}[m.focus]
	focusInd := styleValue.Render("[" + focus + "]")

	var ctx []string
	switch m.focus {
	case zoneList:
		ctx = []string{hint("↑/↓", "browse"), hint("enter", "resume")}
	case zonePreview:
		ctx = []string{hint("↑/↓", "scroll")}
	case zoneOptions:
		ctx = []string{hint("←/→", "pick"), hint("↑/↓", "change"), hint("enter", "toggle")}
	}
	common := []string{hint("tab", "focus"), hint("ctrl+o", "density"), hint("ctrl+c", "quit")}

	sep := styleDim.Render("   ·   ")
	join := func(parts []string) string { return strings.Join(parts, "  ") }
	return focusInd + sep + join(ctx) + sep + join(common)
}

// --- rendering helpers ------------------------------------------------------

// renderDivider returns the vertical column between the panes. Its first row is
// a "┬" junction aligning with the panes' top borders; the rest are "│". Its
// height (bodyHeight+1) matches a pane block (top border + content).
func renderDivider(bodyHeight int) string {
	lines := make([]string, bodyHeight+1)
	lines[0] = styleDivider.Render("┬")
	bar := styleDivider.Render("│")
	for i := 1; i <= bodyHeight; i++ {
		lines[i] = bar
	}
	return strings.Join(lines, "\n")
}

func padLines(lines []string, height int) []string {
	if len(lines) > height {
		return lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

func locationLabel(repoRoot, cwd string) string {
	if repoRoot != "" {
		return homeRelative(repoRoot)
	}
	if cwd != "" {
		return homeRelative(cwd)
	}
	return "(unknown location)"
}

func homeRelative(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
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

func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// wrap breaks text into lines no wider than width (word-wrapped, hard-broken
// for over-long tokens).
func wrap(s string, width int) []string {
	if width < 8 {
		width = 8
	}
	if s == "" {
		return nil
	}
	var lines []string
	var cur string
	for _, word := range strings.Fields(s) {
		for len(word) > width {
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			lines = append(lines, word[:width])
			word = word[width:]
		}
		switch {
		case cur == "":
			cur = word
		case len(cur)+1+len(word) <= width:
			cur += " " + word
		default:
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// truncateMessageLines splits a long message into first N and last N lines
// with an ellipsis separator. If the message has ≤ 2*N lines, it's returned as-is.
func truncateMessageLines(text string, n int) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= 2*n {
		return text
	}
	var sb strings.Builder
	// First n lines
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(lines[i])
	}
	sb.WriteString("\n")
	sb.WriteString(styleEllipsis.Render("  …  "))
	sb.WriteString("\n")
	// Last n lines
	start := len(lines) - n
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(lines[start+i])
	}
	return sb.String()
}

func truncate(s string, maxLen int) string {
	if maxLen < 1 {
		maxLen = 1
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
