package ui

import (
	"fmt"
	"maps"
	"slices"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// Gradient maps a fraction in [0,1] to a pre-computed style.
type Gradient struct {
	steps [101]lipgloss.Style
}

// NewGradient interpolates in Lab space between evenly spaced hex stops.
func NewGradient(hex ...string) Gradient {
	stops := make([]colorful.Color, len(hex))
	for i, h := range hex {
		c, err := colorful.Hex(h)
		if err != nil {
			panic(fmt.Sprintf("invalid gradient color %q: %v", h, err))
		}
		stops[i] = c
	}

	var g Gradient
	for i := range g.steps {
		f := float64(i) / 100
		var c colorful.Color
		if len(stops) == 1 {
			c = stops[0]
		} else {
			pos := f * float64(len(stops)-1)
			j := min(int(pos), len(stops)-2)
			c = stops[j].BlendLab(stops[j+1], pos-float64(j)).Clamped()
		}
		g.steps[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
	}
	return g
}

// At returns the style for fraction f.
func (g Gradient) At(f float64) lipgloss.Style {
	return g.steps[int(clamp01(f)*100+0.5)]
}

// Theme is a complete color scheme.
type Theme struct {
	Name string

	Text     lipgloss.Style
	Dim      lipgloss.Style
	Title    lipgloss.Style
	Hi       lipgloss.Style
	Warn     lipgloss.Style
	Selected lipgloss.Style
	Header   lipgloss.Style

	GPUBox, MemBox, PwrBox, PCIeBox, ProcBox lipgloss.Style

	Util, Mem, Temp, Power, Rx, Tx Gradient
}

type palette struct {
	text, dim, title, hi, warn, selFg, selBg string
	gpuBox, memBox, pwrBox, pcieBox, procBox string
	util, mem, temp, power, rx, tx           []string
}

var palettes = map[string]palette{
	"default": {
		text: "#cccccc", dim: "#404040", title: "#eeeeee", hi: "#b54040", warn: "#e5c07b",
		selFg: "#ecffec", selBg: "#0b4f2a",
		gpuBox: "#3d7b46", memBox: "#8a882e", pwrBox: "#a0522d", pcieBox: "#423ba5", procBox: "#923535",
		util:  []string{"#77ca9b", "#cbc06c", "#dc4c4c"},
		mem:   []string{"#4897d4", "#5474e8", "#9b59d0"},
		temp:  []string{"#4897d4", "#5474e8", "#ff40b6"},
		power: []string{"#f0c674", "#ef8f3a", "#e04040"},
		rx:    []string{"#3d3d8a", "#7a5fd1", "#d27aff"},
		tx:    []string{"#3d6a8a", "#5fb0d1", "#7af0ff"},
	},
	"nord": {
		text: "#d8dee9", dim: "#4c566a", title: "#eceff4", hi: "#88c0d0", warn: "#ebcb8b",
		selFg: "#eceff4", selBg: "#4c566a",
		gpuBox: "#5e81ac", memBox: "#81a1c1", pwrBox: "#b48ead", pcieBox: "#88c0d0", procBox: "#8fbcbb",
		util:  []string{"#a3be8c", "#ebcb8b", "#bf616a"},
		mem:   []string{"#8fbcbb", "#88c0d0", "#5e81ac"},
		temp:  []string{"#88c0d0", "#ebcb8b", "#bf616a"},
		power: []string{"#ebcb8b", "#d08770", "#bf616a"},
		rx:    []string{"#5e81ac", "#81a1c1", "#88c0d0"},
		tx:    []string{"#b48ead", "#c895bf", "#e0a6d6"},
	},
	"dracula": {
		text: "#f8f8f2", dim: "#44475a", title: "#f8f8f2", hi: "#ff79c6", warn: "#f1fa8c",
		selFg: "#f8f8f2", selBg: "#44475a",
		gpuBox: "#bd93f9", memBox: "#8be9fd", pwrBox: "#ffb86c", pcieBox: "#50fa7b", procBox: "#ff79c6",
		util:  []string{"#50fa7b", "#f1fa8c", "#ff5555"},
		mem:   []string{"#8be9fd", "#bd93f9", "#ff79c6"},
		temp:  []string{"#8be9fd", "#f1fa8c", "#ff5555"},
		power: []string{"#f1fa8c", "#ffb86c", "#ff5555"},
		rx:    []string{"#6272a4", "#bd93f9", "#ff79c6"},
		tx:    []string{"#6272a4", "#8be9fd", "#50fa7b"},
	},
	"gruvbox": {
		text: "#ebdbb2", dim: "#504945", title: "#fbf1c7", hi: "#fe8019", warn: "#fabd2f",
		selFg: "#fbf1c7", selBg: "#665c54",
		gpuBox: "#98971a", memBox: "#d79921", pwrBox: "#d65d0e", pcieBox: "#458588", procBox: "#cc241d",
		util:  []string{"#b8bb26", "#fabd2f", "#fb4934"},
		mem:   []string{"#83a598", "#458588", "#b16286"},
		temp:  []string{"#83a598", "#fabd2f", "#fb4934"},
		power: []string{"#fabd2f", "#fe8019", "#fb4934"},
		rx:    []string{"#458588", "#83a598", "#8ec07c"},
		tx:    []string{"#b16286", "#d3869b", "#fe8019"},
	},
}

// ThemeNames lists the built-in themes in alphabetical order.
func ThemeNames() []string {
	return slices.Sorted(maps.Keys(palettes))
}

// LoadTheme returns a built-in theme by name.
func LoadTheme(name string) (Theme, error) {
	p, ok := palettes[name]
	if !ok {
		return Theme{}, fmt.Errorf("unknown theme %q (available: %v)", name, ThemeNames())
	}
	fg := func(hex string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)) }
	return Theme{
		Name:     name,
		Text:     fg(p.text),
		Dim:      fg(p.dim),
		Title:    fg(p.title).Bold(true),
		Hi:       fg(p.hi).Bold(true),
		Warn:     fg(p.warn).Bold(true),
		Selected: fg(p.selFg).Background(lipgloss.Color(p.selBg)).Bold(true),
		Header:   fg(p.title).Bold(true),
		GPUBox:   fg(p.gpuBox),
		MemBox:   fg(p.memBox),
		PwrBox:   fg(p.pwrBox),
		PCIeBox:  fg(p.pcieBox),
		ProcBox:  fg(p.procBox),
		Util:     NewGradient(p.util...),
		Mem:      NewGradient(p.mem...),
		Temp:     NewGradient(p.temp...),
		Power:    NewGradient(p.power...),
		Rx:       NewGradient(p.rx...),
		Tx:       NewGradient(p.tx...),
	}, nil
}

func (t Theme) next() Theme {
	names := ThemeNames()
	i := slices.Index(names, t.Name)
	next, _ := LoadTheme(names[(i+1)%len(names)])
	return next
}

func clamp01(v float64) float64 { return min(max(v, 0), 1) }
