package ui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

var plain = NewGradient("#ffffff")

func TestGraph(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		width  int
		height int
		down   bool
		want   []string
	}{
		{"empty", nil, 3, 1, false, []string{"   "}},
		{"full", []float64{1, 1}, 1, 2, false, []string{"⣿", "⣿"}},
		{"half", []float64{0.5, 0.5}, 1, 2, false, []string{" ", "⣿"}},
		{"ramp", []float64{0.25, 1}, 1, 1, false, []string{"⣸"}},
		{"tiny value is visible", []float64{0.001}, 1, 1, false, []string{"⢀"}},
		{"hanging", []float64{0.5, 0.5}, 1, 2, true, []string{"⣿", " "}},
		{"newest right", []float64{1, 1, 0, 0}, 1, 1, false, []string{" "}},
		{"left padded", []float64{1}, 2, 1, false, []string{" ⢸"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Graph(tt.values, tt.width, tt.height, plain, tt.down)
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Fatalf("Graph = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMeter(t *testing.T) {
	got := Meter(0.5, 10, plain, lipgloss.NewStyle())
	if w := lipgloss.Width(got); w != 10 {
		t.Fatalf("width = %d, want 10", w)
	}
	if Meter(2, 4, plain, lipgloss.NewStyle()) != "■■■■" {
		t.Fatal("over-range fraction should clamp to full")
	}
}

func TestBox(t *testing.T) {
	s := plainStyle()
	out := Box("gpu", "keys", []string{"hello", strings.Repeat("x", 50)}, 20, 4, s, s)
	lines := strings.Split(out, "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w != 20 {
			t.Errorf("line %d width = %d, want 20: %q", i, w, l)
		}
	}
	if !strings.HasPrefix(lines[0], "╭─┐gpu┌") {
		t.Errorf("unexpected title line %q", lines[0])
	}
	if !strings.Contains(lines[3], "┘keys└") {
		t.Errorf("unexpected footer line %q", lines[3])
	}
}

func TestBytes(t *testing.T) {
	tests := map[uint64]string{
		0:             "0B",
		1023:          "1023B",
		1024:          "1.0K",
		1536:          "1.5K",
		12 << 30:      "12.0G",
		512 << 20:     "512M",
		1<<40 + 1<<39: "1.5T",
	}
	for in, want := range tests {
		if got := Bytes(in); got != want {
			t.Errorf("Bytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFitAndSpread(t *testing.T) {
	if got := Fit("abcdef", 3); got != "abc" {
		t.Errorf("Fit truncate = %q", got)
	}
	if got := Fit("ab", 4); got != "ab  " {
		t.Errorf("Fit pad = %q", got)
	}
	if got := Spread("a", "b", 5); got != "a   b" {
		t.Errorf("Spread = %q", got)
	}
}

func TestThemes(t *testing.T) {
	for _, name := range ThemeNames() {
		if _, err := LoadTheme(name); err != nil {
			t.Errorf("LoadTheme(%q): %v", name, err)
		}
	}
	if _, err := LoadTheme("nope"); err == nil {
		t.Error("expected error for unknown theme")
	}
}

func plainStyle() lipgloss.Style { return lipgloss.NewStyle() }
