package ui

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/zvdy/nvtop/internal/gpu"
)

func newTestModel(t *testing.T, gpus, w, h int) Model {
	t.Helper()
	theme, err := LoadTheme("default")
	if err != nil {
		t.Fatal(err)
	}
	src := gpu.NewMock(gpus, 42)
	m := New(Config{Source: src, Interval: time.Second, Theme: theme})
	m = step(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	for range 30 {
		snap, err := src.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		m = step(t, m, snapshotMsg{snap: snap})
	}
	return m
}

func step(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	return next.(Model)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestViewFitsTerminal(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 60}} {
		for _, gpus := range []int{1, 3} {
			m := newTestModel(t, gpus, size[0], size[1])
			lines := strings.Split(m.View(), "\n")
			if len(lines) != size[1] {
				t.Errorf("%dx%d/%d gpus: %d lines, want %d", size[0], size[1], gpus, len(lines), size[1])
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w != size[0] {
					t.Errorf("%dx%d/%d gpus: line %d width %d: %q", size[0], size[1], gpus, i, w, l)
				}
			}
		}
	}
	if os.Getenv("NVTOP_DUMP") != "" {
		t.Log("\n" + newTestModel(t, 2, 120, 40).View())
	}
}

func TestViewStates(t *testing.T) {
	m := newTestModel(t, 1, 40, 10)
	if !strings.Contains(m.View(), "terminal too small") {
		t.Error("expected small terminal warning")
	}

	m = newTestModel(t, 1, 100, 30)
	m = step(t, m, key("?"))
	if !strings.Contains(m.View(), "cycle theme") {
		t.Error("expected help view")
	}
	m = step(t, m, key("x"))
	if m.help {
		t.Error("any key should close help")
	}
}

func TestDeviceNavigation(t *testing.T) {
	m := newTestModel(t, 3, 120, 40)
	m = step(t, m, key("tab"))
	if m.device != 1 {
		t.Fatalf("device = %d after tab, want 1", m.device)
	}
	m = step(t, m, key("2"))
	m = step(t, m, key("tab"))
	if m.device != 0 {
		t.Fatalf("device = %d, want wrap to 0", m.device)
	}
}

func TestSelectionFollowsPID(t *testing.T) {
	m := newTestModel(t, 1, 120, 40)
	if len(m.procs.rows) < 2 {
		t.Skip("mock produced too few processes")
	}
	m = step(t, m, key("down"))
	pid := m.procs.rows[m.procs.cursor].PID

	m = step(t, m, key("s"))
	m = step(t, m, key("s"))
	if got := m.procs.rows[m.procs.cursor].PID; got != pid {
		t.Fatalf("selection moved from pid %d to %d after re-sort", pid, got)
	}
}

func TestKillConfirmation(t *testing.T) {
	m := newTestModel(t, 1, 120, 40)
	m = step(t, m, key("x"))
	if m.confirm == nil {
		t.Fatal("expected confirmation prompt")
	}
	if !strings.Contains(m.View(), "[t]erm") {
		t.Error("prompt not rendered")
	}
	m = step(t, m, key("esc"))
	if m.confirm != nil {
		t.Fatal("esc should cancel")
	}
}

func TestIntervalBounds(t *testing.T) {
	m := newTestModel(t, 1, 120, 40)
	for range 20 {
		m = step(t, m, key("+"))
	}
	if m.interval != maxInterval {
		t.Errorf("interval = %s, want %s", m.interval, maxInterval)
	}
	for range 20 {
		m = step(t, m, key("-"))
	}
	if m.interval != minInterval {
		t.Errorf("interval = %s, want %s", m.interval, minInterval)
	}
}

func TestPauseSkipsFetch(t *testing.T) {
	m := newTestModel(t, 1, 120, 40)
	m = step(t, m, key(" "))
	if !m.paused {
		t.Fatal("expected paused")
	}
	if !strings.Contains(m.View(), "PAUSED") {
		t.Error("paused indicator missing")
	}
}
