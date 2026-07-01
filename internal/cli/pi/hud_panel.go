/**
 * Component: Pi HUD Sidebar Panel
 * Block-UUID: d3a8e472-6c19-4f5b-8e20-9b1c7a04f8d5
 * Parent-UUID: c1f6a930-5e24-4b88-9d07-2a4b8e15c6f3
 * Version: 2.1.0
 * Description: Implements `gsc pi --hud-panel <id>`, the narrow tmux sidebar beside a running pi session. Polls the SQLite mirror on a tick and renders pi-brains-style sections: a context-token gauge with bitmap glyphs (from the last assistant usage in raw_line), model/provider, a touched-file tree grouped by repo with an outside-repo count, and the active GitSense Brains for the repo (via `gsc brains --json`). Uses a flat, non-distracting white/grey palette (no bold, no accent colors).
 * Language: Go
 * Created-at: 2026-06-19T00:00:00Z
 * Authors: claude-opus-4-8 (v1.0.0, v2.0.0, v2.1.0)
 */


package pi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
)

// hudRefresh is how often the sidebar re-reads the mirror. The numbers are only
// as fresh as the sync watcher's last import, so polling faster buys little.
const hudRefresh = 2 * time.Second

// hudPadLeft insets the whole panel so text doesn't touch the pane's left edge.
const hudPadLeft = 2

// runHudPanel renders the sidebar for a single session until the user quits or
// the pane closes. Invoked by the tmux split in execTmuxHud, not directly.
func runHudPanel(dbPath, sessionID string) error {
	resolvedDB, err := resolvePiSessionsDBPath(dbPath)
	if err != nil {
		return err
	}
	self, _ := os.Executable() // used to shell `gsc brains --json` per repo
	m := hudPanelModel{dbPath: resolvedDB, sessionID: sessionID, self: self, width: hudPanelWidth, height: 24}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

type hudPanelModel struct {
	dbPath    string
	sessionID string
	self      string

	show    *pisessions.ShowResult
	usage   *pisessions.SessionUsage
	files   []pisessions.TouchedFile
	brains  []string
	err     error
	updated time.Time

	width  int
	height int
}

type hudTickMsg time.Time

type hudDataMsg struct {
	show   *pisessions.ShowResult
	usage  *pisessions.SessionUsage
	files  []pisessions.TouchedFile
	brains []string
	err    error
}

func (m hudPanelModel) Init() tea.Cmd {
	return tea.Batch(m.fetchCmd(), hudTick())
}

func (m hudPanelModel) fetchCmd() tea.Cmd {
	dbPath, id, self := m.dbPath, m.sessionID, m.self
	return func() tea.Msg {
		show, err := pisessions.Show(context.Background(), dbPath, id)
		if err != nil {
			return hudDataMsg{err: err}
		}
		usage, _ := pisessions.LastUsage(context.Background(), dbPath, id)
		files, _ := pisessions.TouchedFiles(context.Background(), dbPath, id)
		return hudDataMsg{show: show, usage: usage, files: files, brains: fetchBrains(self, show.RepoRoot)}
	}
}

func hudTick() tea.Cmd {
	return tea.Tick(hudRefresh, func(t time.Time) tea.Msg { return hudTickMsg(t) })
}

// fetchBrains lists the active GitSense Brain names for a repo by shelling out to
// `gsc brains --json` in that directory. Best-effort: any failure yields no list.
func fetchBrains(self, repoRoot string) []string {
	if self == "" || repoRoot == "" {
		return nil
	}
	cmd := exec.Command(self, "brains", "--json")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var parsed struct {
		Databases []struct {
			Name string `json:"name"`
		} `json:"databases"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil
	}
	names := make([]string, 0, len(parsed.Databases))
	for _, d := range parsed.Databases {
		if d.Name != "" {
			names = append(names, d.Name)
		}
	}
	return names
}

func (m hudPanelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case hudDataMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.show, m.usage, m.files, m.brains = msg.show, msg.usage, msg.files, msg.brains
		m.err = nil
		m.updated = time.Now()
		return m, nil
	case hudTickMsg:
		return m, tea.Batch(m.fetchCmd(), hudTick())
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

// HUD styling is intentionally flat: a slightly-dimmed white for primary text,
// a dimmer grey for labels/secondary, no bold and no accent colors. A HUD should
// inform without competing with the conversation.
var (
	hudSection = lipgloss.NewStyle().Foreground(lipgloss.Color("250")) // section headers (white)
	hudBright  = lipgloss.NewStyle().Foreground(lipgloss.Color("250")) // primary values (white)
	hudDimSt   = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // labels / secondary
	hudGlyph   = lipgloss.NewStyle().Foreground(lipgloss.Color("244")) // dimmed glyph block
	hudOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // brain check (dim)
	hudErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("131")) // muted red, errors only
	hudPad     = lipgloss.NewStyle().PaddingLeft(hudPadLeft)           // left inset for the panel
)

func (m hudPanelModel) View() string {
	w := m.width
	if w < 12 {
		w = hudPanelWidth
	}
	// Reserve the left inset (and a one-column right gap) from the content width.
	inner := w - hudPadLeft - 1

	var b strings.Builder
	b.WriteString(hudSection.Render("PI HUD"))
	b.WriteString("\n" + hudDimSt.Render(strings.Repeat("─", inner)) + "\n")

	if m.err != nil {
		b.WriteString("\n" + hudErr.Render(truncate("error: "+m.err.Error(), inner)))
		return hudPad.Render(b.String())
	}
	if m.show == nil {
		b.WriteString("\n" + hudDimSt.Render("loading…"))
		return hudPad.Render(b.String())
	}

	m.writeContext(&b, inner)
	m.writeModel(&b, inner)
	m.writeFiles(&b, inner)
	m.writeBrains(&b, inner)

	if !m.updated.IsZero() {
		b.WriteString("\n" + hudDimSt.Render("updated "+m.updated.Format("15:04:05")))
	}
	return hudPad.Render(b.String())
}

func (m hudPanelModel) writeContext(b *strings.Builder, inner int) {
	b.WriteString("\n" + hudSection.Render("CONTEXT") + "\n")
	if m.usage == nil {
		b.WriteString(hudDimSt.Render("—  (no response yet)") + "\n")
		return
	}
	ctx := m.usage.ContextTokens()
	abbr := abbrevTokens(ctx)

	// Bitmap glyphs for the abbreviated context size, when they fit.
	glyph := renderGlyphLines(abbr)
	if len(glyph) > 0 && lipgloss.Width(glyph[0]) <= inner {
		for _, line := range glyph {
			b.WriteString(hudGlyph.Render(line) + "\n")
		}
	}

	if win := contextWindow(m.show.Model); win > 0 {
		pct := int(math.Round(float64(ctx) / float64(win) * 100))
		b.WriteString(hudBright.Render(fmt.Sprintf("%s / %s · %d%%", abbr, abbrevTokens(win), pct)) + "\n")
	} else {
		b.WriteString(hudBright.Render(abbr+" tokens") + "\n")
	}
	b.WriteString(hudDimSt.Render(fmt.Sprintf("in %s · out %s", abbrevTokens(m.usage.InputTokens), abbrevTokens(m.usage.OutputTokens))) + "\n")
	b.WriteString(hudDimSt.Render(fmt.Sprintf("cache %s · $%.4f", abbrevTokens(m.usage.CacheRead), m.usage.CostTotal)) + "\n")
}

func (m hudPanelModel) writeModel(b *strings.Builder, inner int) {
	if m.show.Model == "" {
		return
	}
	b.WriteString("\n" + hudSection.Render("MODEL") + "\n")
	b.WriteString(hudBright.Render(truncate(m.show.Model, inner)) + "\n")
	if m.show.Provider != "" {
		b.WriteString(hudDimSt.Render(truncate(m.show.Provider, inner)) + "\n")
	}
}

func (m hudPanelModel) writeFiles(b *strings.Builder, inner int) {
	var inRepo, outside []pisessions.TouchedFile
	for _, f := range m.files {
		if f.FilePathRel != "" {
			inRepo = append(inRepo, f)
		} else {
			outside = append(outside, f)
		}
	}

	b.WriteString("\n" + hudSection.Render(fmt.Sprintf("FILES TRACKED (%d)", len(m.files))) + "\n")
	if m.show.RepoRoot != "" {
		b.WriteString(hudDimSt.Render(truncate(homeRelative(m.show.RepoRoot), inner)) + "\n")
	}

	rels := make([]string, 0, len(inRepo))
	for _, f := range inRepo {
		rels = append(rels, f.FilePathRel)
	}
	// Budget the tree so the panel does not overflow short terminals.
	budget := m.height - 18
	if budget < 4 {
		budget = 4
	}
	for _, line := range renderFileTree(rels, budget) {
		b.WriteString(hudBright.Render(truncate(line, inner)) + "\n")
	}
	if len(outside) > 0 {
		b.WriteString(hudDimSt.Render(fmt.Sprintf("outside repo: %d", len(outside))) + "\n")
	}
}

func (m hudPanelModel) writeBrains(b *strings.Builder, inner int) {
	if len(m.brains) == 0 {
		return
	}
	b.WriteString("\n" + hudSection.Render("BRAINS") + "\n")
	for _, name := range m.brains {
		b.WriteString(hudOK.Render("✓ ") + hudDimSt.Render(truncate(name, inner-2)) + "\n")
	}
}

// --- token formatting + glyphs --------------------------------------------

// abbrevTokens renders a token count compactly: 36910 -> "37K", 1_200_000 -> "1.2M".
func abbrevTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1000:
		return fmt.Sprintf("%dK", int(math.Round(float64(n)/1000)))
	default:
		return fmt.Sprintf("%d", n)
	}
}

// contextWindow is a best-effort lookup of a model's context window for the
// percent gauge. Out-of-process we cannot read Pi's model registry, so this maps
// common families; unknown models render the raw token count without a percent.
func contextWindow(model string) int {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "claude"), strings.Contains(m, "opus"), strings.Contains(m, "sonnet"), strings.Contains(m, "haiku"):
		return 200000
	case strings.Contains(m, "gpt"), strings.Contains(m, "o1"), strings.Contains(m, "o3"):
		return 128000
	case strings.Contains(m, "gemini"):
		return 1000000
	default:
		return 0
	}
}

// glyphFont is a 3x5 bitmap font for the context-size readout.
var glyphFont = map[rune][5]string{
	'0': {"███", "█ █", "█ █", "█ █", "███"},
	'1': {" █ ", "██ ", " █ ", " █ ", "███"},
	'2': {"███", "  █", "███", "█  ", "███"},
	'3': {"███", "  █", "███", "  █", "███"},
	'4': {"█ █", "█ █", "███", "  █", "  █"},
	'5': {"███", "█  ", "███", "  █", "███"},
	'6': {"███", "█  ", "███", "█ █", "███"},
	'7': {"███", "  █", "  █", "  █", "  █"},
	'8': {"███", "█ █", "███", "█ █", "███"},
	'9': {"███", "█ █", "███", "  █", "███"},
	'K': {"█ █", "██ ", "█  ", "██ ", "█ █"},
	'M': {"█ █", "███", "███", "█ █", "█ █"},
	'.': {"   ", "   ", "   ", "   ", " █ "},
	' ': {"   ", "   ", "   ", "   ", "   "},
}

// renderGlyphLines lays a string out in the 3x5 bitmap font, returning 5 lines.
func renderGlyphLines(s string) []string {
	rows := make([]string, 5)
	for _, ch := range s {
		g, ok := glyphFont[ch]
		if !ok {
			g = glyphFont[' ']
		}
		for i := 0; i < 5; i++ {
			rows[i] += g[i] + " "
		}
	}
	for i := range rows {
		rows[i] = strings.TrimRight(rows[i], " ")
	}
	return rows
}

// --- file tree ------------------------------------------------------------

type fileTreeNode struct {
	children map[string]*fileTreeNode
	order    []string
}

func newFileTreeNode() *fileTreeNode {
	return &fileTreeNode{children: map[string]*fileTreeNode{}}
}

func (n *fileTreeNode) add(parts []string) {
	cur := n
	for _, p := range parts {
		child, ok := cur.children[p]
		if !ok {
			child = newFileTreeNode()
			cur.children[p] = child
			cur.order = append(cur.order, p)
		}
		cur = child
	}
}

// renderFileTree builds a directory tree from repo-relative paths and renders it
// with box-drawing connectors, truncated to at most budget lines.
func renderFileTree(paths []string, budget int) []string {
	if len(paths) == 0 {
		return nil
	}
	root := newFileTreeNode()
	for _, p := range paths {
		root.add(strings.Split(p, "/"))
	}
	lines := walkFileTree(root, "")
	if len(lines) > budget {
		more := len(lines) - (budget - 1)
		lines = append(lines[:budget-1], hudDimSt.Render(fmt.Sprintf("… %d more", more)))
	}
	return lines
}

func walkFileTree(node *fileTreeNode, prefix string) []string {
	sort.Strings(node.order)
	var lines []string
	for i, name := range node.order {
		last := i == len(node.order)-1
		branch := "├─ "
		childPrefix := prefix + "│  "
		if last {
			branch = "└─ "
			childPrefix = prefix + "   "
		}
		lines = append(lines, prefix+branch+name)
		if child := node.children[name]; len(child.order) > 0 {
			lines = append(lines, walkFileTree(child, childPrefix)...)
		}
	}
	return lines
}
