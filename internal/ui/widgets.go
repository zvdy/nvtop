package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Braille dot bits for the left and right column of a cell, top to bottom.
var (
	brailleLeft  = [4]rune{0x01, 0x02, 0x04, 0x40}
	brailleRight = [4]rune{0x08, 0x10, 0x20, 0x80}
)

// Graph renders values in [0,1] as a filled braille area chart with two
// samples per column and four levels per row. The newest sample is on the
// right. When down is set the chart hangs from the top edge instead.
func Graph(values []float64, width, height int, grad Gradient, down bool) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	samples := make([]float64, width*2)
	for i := range samples {
		samples[i] = math.NaN()
	}
	if n := min(len(values), len(samples)); n > 0 {
		copy(samples[len(samples)-n:], values[len(values)-n:])
	}

	dots := height * 4
	levels := make([]int, len(samples))
	for i, v := range samples {
		if math.IsNaN(v) {
			continue
		}
		levels[i] = int(math.Round(clamp01(v) * float64(dots)))
		if levels[i] == 0 && v > 0 {
			levels[i] = 1
		}
	}

	lines := make([]string, height)
	cells := make([]rune, width)
	for row := range height {
		depth := height - 1 - row
		if down {
			depth = row
		}
		for col := range width {
			cells[col] = brailleCell(levels[col*2], levels[col*2+1], depth, down)
		}
		lines[row] = grad.At(float64(depth+1) / float64(height)).Render(string(cells))
	}
	return lines
}

func brailleCell(left, right, depth int, down bool) rune {
	var r rune
	for side, level := range [2]int{left, right} {
		bits := brailleLeft
		if side == 1 {
			bits = brailleRight
		}
		fill := min(max(level-depth*4, 0), 4)
		for i := range fill {
			if down {
				r |= bits[i]
			} else {
				r |= bits[3-i]
			}
		}
	}
	if r == 0 {
		return ' '
	}
	return 0x2800 + r
}

// Meter renders a horizontal bar where each filled cell is colored by its
// position along the gradient.
func Meter(frac float64, width int, grad Gradient, empty lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	filled := int(math.Round(clamp01(frac) * float64(width)))
	var sb strings.Builder
	for i := range filled {
		sb.WriteString(grad.At(float64(i) / float64(max(width-1, 1))).Render("■"))
	}
	if filled < width {
		sb.WriteString(empty.Render(strings.Repeat("■", width-filled)))
	}
	return sb.String()
}

// Box draws a rounded frame with an embedded title in the btop style.
// Body lines are truncated or padded to fit exactly.
func Box(title, footer string, body []string, width, height int, border, titleStyle lipgloss.Style) string {
	if width < 4 || height < 2 {
		return ""
	}
	inner := width - 2
	var sb strings.Builder

	sb.WriteString(border.Render("╭─"))
	used := 2
	if title != "" && inner > 6 {
		t := ansi.Truncate(title, inner-4, "…")
		sb.WriteString(border.Render("┐"))
		sb.WriteString(titleStyle.Render(t))
		sb.WriteString(border.Render("┌"))
		used += 2 + lipgloss.Width(t)
	}
	sb.WriteString(border.Render(strings.Repeat("─", max(width-used-1, 0)) + "╮"))
	sb.WriteByte('\n')

	edge := border.Render("│")
	for i := range height - 2 {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		sb.WriteString(edge)
		sb.WriteString(Fit(line, inner))
		sb.WriteString(edge)
		sb.WriteByte('\n')
	}

	if footer != "" && lipgloss.Width(footer)+6 < inner {
		pad := inner - lipgloss.Width(footer) - 3
		sb.WriteString(border.Render("╰" + strings.Repeat("─", pad) + "┘"))
		sb.WriteString(footer)
		sb.WriteString(border.Render("└─╯"))
	} else {
		sb.WriteString(border.Render("╰" + strings.Repeat("─", inner) + "╯"))
	}
	return sb.String()
}

// Fit truncates or right-pads s to exactly width visible cells.
func Fit(s string, width int) string {
	w := lipgloss.Width(s)
	switch {
	case w > width:
		return ansi.Truncate(s, width, "")
	case w < width:
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// Spread places left and right on one line of the given width.
func Spread(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return Fit(left+" "+right, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

// Bytes formats a byte count with binary prefixes.
func Bytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit && exp < 5; n /= unit {
		div *= unit
		exp++
	}
	v := float64(b) / float64(div)
	prefix := "KMGTPE"[exp]
	if v >= 100 {
		return fmt.Sprintf("%.0f%c", v, prefix)
	}
	return fmt.Sprintf("%.1f%c", v, prefix)
}

// Rate formats a throughput in bytes per second.
func Rate(bps uint64) string { return Bytes(bps) + "/s" }
