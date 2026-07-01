/**
 * Component: Pi HUD Launcher
 * Block-UUID: e2a7c5f9-3b81-4d06-9f24-7c1e8b3a5d60
 * Parent-UUID: a3f1c9d2-7b64-4e10-9c3a-2d5e8f0b1a76
 * Version: 1.4.0
 * Description: Implements `gsc pi --hud`: reuses the resume picker to choose a session, then launches a tmux split with pi on the left and the gsc HUD sidebar (`gsc pi --hud-panel`) pinned to a fixed width on the right. On the isolated tmux server it sets extended-keys=always (so pi's modified-key warning clears and modified keys pass through) and mouse=off (pi captures mouse events via bubbletea, so tmux mouse mode causes scroll wheel to be interpreted as up/down navigation; disabling lets the terminal handle scroll natively), tears the whole session down when pi exits, and only sets a start directory when the session has one.
 * Language: Go
 * Created-at: 2026-06-19T00:00:00Z
 * Authors: claude-opus-4-8 (v1.0.0, v1.1.0, v1.2.0, v1.3.0), MiMo-v2.5-Pro (v1.4.0)
 */


package pi

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const (
	// hudPanelWidth is the column width reserved for the HUD sidebar. A couple
	// of columns beyond the usable text leave room for the pane border.
	hudPanelWidth = 52
	// hudSocket and hudSession isolate the HUD's tmux server/session so it never
	// touches the user's normal tmux config, keybindings, or sessions.
	hudSocket  = "gsc"
	hudSession = "gsc-pi"
)

// runHudPicker shows the resume picker and, on selection, launches a tmux HUD
// (pi + sidebar) for the chosen session instead of resuming pi directly.
func runHudPicker(dbPath string, passthrough []string) error {
	id, dir, err := pickSession(dbPath)
	if err != nil || id == "" {
		return err
	}
	return execTmuxHud(dbPath, id, dir, passthrough)
}

// execTmuxHud builds a detached tmux session with two side-by-side panes — pi on
// the left, the gsc HUD sidebar on the right pinned to hudPanelWidth — then
// replaces the current process by attaching to it. pi is focused on attach.
func execTmuxHud(dbPath, sessionID, dir string, passthrough []string) error {
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found on PATH: %w", err)
	}
	if _, err := exec.LookPath("pi"); err != nil {
		return fmt.Errorf("pi executable not found on PATH: %w", err)
	}
	if os.Getenv("TMUX") != "" {
		return fmt.Errorf("already inside tmux; run `gsc pi --hud` from a plain terminal")
	}

	// The sidebar reads the same mirror as the picker. Resolve it to an explicit
	// path so the panel process does not have to re-derive the default.
	resolvedDB, err := resolvePiSessionsDBPath(dbPath)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate gsc executable: %w", err)
	}

	// When pi exits, kill the whole session so the HUD pane dies with it and the
	// attach returns immediately — otherwise the lingering sidebar pane keeps the
	// session alive and the user has to quit a second time.
	piCmd := shellJoin(append([]string{"pi", "--session", sessionID}, passthrough...)) +
		"; " + shellJoin([]string{tmux, "-L", hudSocket, "kill-session", "-t", hudSession})
	hudCmd := shellJoin([]string{self, "pi", "--hud-panel", sessionID, "--db", resolvedDB})
	width := strconv.Itoa(hudPanelWidth)

	run := func(args ...string) error {
		full := append([]string{"-L", hudSocket}, args...)
		if out, err := exec.Command(tmux, full...).CombinedOutput(); err != nil {
			return fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	// Clear any stale session from a previous run, then build the split detached
	// (a single exec cannot run the multi-step tmux setup before attaching).
	_ = run("kill-session", "-t", hudSession)
	// Configure the private server *before* the session starts so pi inherits it.
	// Use extended-keys=always: plain `on` does not reliably stick (it can
	// normalize back to off), leaving pi's modified-key warning up; `always`
	// forces extended keys through. mouse=off lets the terminal handle scroll
	// natively (pi captures mouse events via bubbletea, so tmux mouse mode
	// causes scroll wheel to be interpreted as up/down navigation). This server
	// is private (-L gsc), so it never touches the user's normal tmux config.
	_ = run("start-server")
	_ = run("set-option", "-g", "extended-keys", "always")
	_ = run("set-option", "-g", "mouse", "off")
	// Only pass -c when the session has a known directory; tmux errors on -c "".
	newArgs := []string{"new-session", "-d", "-s", hudSession}
	if dir != "" {
		newArgs = append(newArgs, "-c", dir)
	}
	newArgs = append(newArgs, piCmd)
	if err := run(newArgs...); err != nil {
		return err
	}
	if err := run("split-window", "-h", "-l", width, "-t", hudSession, hudCmd); err != nil {
		return err
	}
	// Re-pin the sidebar width whenever the client resizes, and once more now so
	// the first attach (which resizes the detached 80x24 session to the real
	// terminal) does not leave the sidebar reflowed.
	_ = run("set-hook", "-t", hudSession, "client-resized", "resize-pane -t "+hudSession+".1 -x "+width)
	_ = run("resize-pane", "-t", hudSession+".1", "-x", width)
	_ = run("select-pane", "-t", hudSession+".0") // focus pi

	return syscall.Exec(tmux, []string{"tmux", "-L", hudSocket, "attach", "-t", hudSession}, os.Environ())
}

// shellJoin renders argv as a single POSIX-shell command string, since tmux runs
// pane commands through /bin/sh -c. Each argument is single-quoted so spaces and
// metacharacters in paths or passthrough flags are preserved literally.
func shellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = shellQuote(a)
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
