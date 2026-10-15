package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/zvdy/nvtop/internal/gpu"
)

const (
	minWidth  = 80
	minHeight = 24
)

type layout struct {
	header, gpu, mid, proc int
}

func computeLayout(height, devices int) layout {
	l := layout{header: 1}
	if devices > 1 {
		l.header++
	}
	avail := height - l.header
	l.gpu = max(9, avail*40/100)
	l.mid = max(7, avail*26/100)
	l.proc = avail - l.gpu - l.mid
	return l
}

// procRows is the number of process rows visible in the table.
func (m Model) procRows() int {
	return computeLayout(m.height, len(m.snap.Devices)).proc - 3
}

// View implements tea.Model.
func (m Model) View() string {
	switch {
	case m.width == 0:
		return ""
	case m.width < minWidth || m.height < minHeight:
		return m.center(m.theme.Warn.Render(fmt.Sprintf(
			"terminal too small: %dx%d (need %dx%d)", m.width, m.height, minWidth, minHeight)))
	case m.help:
		return m.center(m.helpView())
	case !m.hasSnap && m.err != nil:
		return m.center(m.theme.Hi.Render("error: ") + m.theme.Text.Render(m.err.Error()))
	case !m.hasSnap:
		return m.center(m.theme.Dim.Render("collecting first sample…"))
	}

	l := computeLayout(m.height, len(m.snap.Devices))
	d := m.currentDevice()
	h := m.hist[m.device]

	memW := m.width * 34 / 100
	pwrW := m.width * 33 / 100
	pcieW := m.width - memW - pwrW

	sections := []string{m.headerView()}
	if len(m.snap.Devices) > 1 {
		sections = append(sections, m.deviceStrip())
	}
	sections = append(sections,
		m.gpuBox(d, h, m.width, l.gpu),
		lipgloss.JoinHorizontal(lipgloss.Top,
			m.memBox(d, h, memW, l.mid),
			m.powerBox(d, h, pwrW, l.mid),
			m.pcieBox(d, h, pcieW, l.mid),
		),
		m.procBox(m.width, l.proc),
	)
	return strings.Join(sections, "\n")
}

func (m Model) center(s string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, s)
}

func (m Model) headerView() string {
	t := m.theme
	sep := t.Dim.Render(" │ ")
	left := t.Hi.Render(" nvtop") + sep + t.Text.Render(m.host)
	if m.snap.Driver != "" {
		left += sep + t.Dim.Render("driver ") + t.Text.Render(m.snap.Driver)
	}
	if m.snap.CUDA != "" {
		left += sep + t.Dim.Render("cuda ") + t.Text.Render(m.snap.CUDA)
	}
	left += sep + t.Dim.Render("every ") + t.Text.Render(m.interval.String())

	right := t.Text.Render(m.snap.Taken.Format("15:04:05")) + " "
	switch {
	case m.err != nil:
		right = t.Hi.Render("error: "+m.err.Error()) + "  " + right
	case m.status != "" && time.Now().Before(m.statusUntil):
		right = t.Warn.Render(m.status) + "  " + right
	}
	if m.paused {
		right = t.Warn.Render("⏸ PAUSED") + "  " + right
	}
	return Spread(left, right, m.width)
}

func (m Model) deviceStrip() string {
	t := m.theme
	var parts []string
	for i, d := range m.snap.Devices {
		label := fmt.Sprintf(" %d ", i)
		if i == m.device {
			label = t.Selected.Render(label)
		} else {
			label = t.Dim.Render(label)
		}
		util := float64(d.Util.V) / 100
		parts = append(parts, label+" "+Meter(util, 8, t.Util, t.Dim)+" "+t.Text.Render(fmt.Sprintf("%3d%%", d.Util.V)))
	}
	return Fit(" "+strings.Join(parts, "   "), m.width)
}

func (m Model) gpuBox(d gpu.Device, h *deviceHistory, w, height int) string {
	t := m.theme
	iw, ih := w-2, height-2
	panelW := min(max(iw*45/100, 36), 54)
	graphW := iw - panelW - 1

	label := t.Title.Render("util ") + t.Util.At(float64(d.Util.V)/100).Render(fmt.Sprintf("%d%%", d.Util.V))
	graph := append([]string{label}, Graph(h.util.Values(), graphW, ih-1, t.Util, false)...)
	panel := m.gpuPanel(d, panelW)

	body := make([]string, ih)
	div := t.Dim.Render("│")
	for i := range body {
		var g, p string
		if i < len(graph) {
			g = graph[i]
		}
		if i < len(panel) {
			p = panel[i]
		}
		body[i] = Fit(g, graphW) + div + Fit(p, panelW)
	}

	title := fmt.Sprintf("gpu%d %s", d.Index, d.Name)
	footer := t.Dim.Render(d.BusID)
	return Box(title, footer, body, w, height, t.GPUBox, t.Title)
}

func (m Model) gpuPanel(d gpu.Device, w int) []string {
	t := m.theme
	const labelW, valueW = 5, 13
	meterW := w - labelW - valueW - 3

	row := func(label string, v float64, ok bool, grad Gradient, value string) string {
		if !ok {
			return " " + t.Title.Render(Fit(label, labelW)) + Meter(0, meterW, grad, t.Dim) + " " + t.Dim.Render(fmt.Sprintf("%*s", valueW, "n/a"))
		}
		return " " + t.Title.Render(Fit(label, labelW)) + Meter(v, meterW, grad, t.Dim) + " " + t.Text.Render(fmt.Sprintf("%*s", valueW, value))
	}
	pct := func(label string, v gpu.Value[uint32], grad Gradient) string {
		return row(label, float64(v.V)/100, v.OK, grad, fmt.Sprintf("%d%%", v.V))
	}

	lines := []string{
		pct("GPU", d.Util, t.Util),
		row("VRAM", d.MemFraction(), d.MemTotal > 0, t.Mem, Bytes(d.MemUsed)+"/"+Bytes(d.MemTotal)),
		row("TEMP", float64(d.Temp.V)/tempCeiling(d), d.Temp.OK, t.Temp, fmt.Sprintf("%d°C", d.Temp.V)),
		row("PWR", d.Power.V/max(d.PowerLimit.V, 1), d.Power.OK, t.Power, fmt.Sprintf("%.0f/%.0fW", d.Power.V, d.PowerLimit.V)),
		pct("FAN", d.Fan, t.Util),
		pct("BW", d.MemUtil, t.Mem),
		pct("ENC", d.EncUtil, t.Rx),
		pct("DEC", d.DecUtil, t.Tx),
	}

	kv := func(k, v string) string { return t.Dim.Render(k) + t.Text.Render(v) }
	clocks := " " + kv("CLK ", mhz(d.ClockGfx, d.ClockGfxMax)) + "  " + kv("MEM ", mhz(d.ClockMem, d.ClockMemMax))
	pstate := "n/a"
	if d.PState.OK {
		pstate = fmt.Sprintf("P%d", d.PState.V)
	}
	extra := " " + kv("PCIe ", "▼"+Rate(d.PCIeRx.V)+" ▲"+Rate(d.PCIeTx.V)) + "  " + kv("PERF ", pstate)
	return append(lines, clocks, extra)
}

func mhz(cur, maxV gpu.Value[uint32]) string {
	switch {
	case !cur.OK:
		return "n/a"
	case maxV.OK && maxV.V > 0:
		return fmt.Sprintf("%d/%dMHz", cur.V, maxV.V)
	default:
		return fmt.Sprintf("%dMHz", cur.V)
	}
}

func (m Model) memBox(d gpu.Device, h *deviceHistory, w, height int) string {
	t := m.theme
	iw, ih := w-2, height-2
	head := Spread(
		t.Title.Render("used ")+t.Mem.At(d.MemFraction()).Render(Bytes(d.MemUsed)),
		t.Dim.Render("free ")+t.Text.Render(Bytes(d.MemTotal-d.MemUsed)), iw)
	body := append([]string{head}, Graph(h.mem.Values(), iw, ih-1, t.Mem, false)...)
	footer := t.Dim.Render(fmt.Sprintf("%.0f%% of %s", d.MemFraction()*100, Bytes(d.MemTotal)))
	return Box("vram", footer, body, w, height, t.MemBox, t.Title)
}

func (m Model) powerBox(d gpu.Device, h *deviceHistory, w, height int) string {
	t := m.theme
	iw, ih := w-2, height-2
	gh := ih - 2
	ph, th := gh-gh/2, gh/2

	pwr := t.Title.Render("pwr ") + t.Dim.Render("n/a")
	if d.Power.OK {
		pwr = t.Title.Render("pwr ") + t.Power.At(d.Power.V/max(d.PowerLimit.V, 1)).Render(fmt.Sprintf("%.0fW", d.Power.V))
	}
	temp := t.Title.Render("temp ") + t.Dim.Render("n/a")
	if d.Temp.OK {
		temp = t.Title.Render("temp ") + t.Temp.At(float64(d.Temp.V)/tempCeiling(d)).Render(fmt.Sprintf("%d°C", d.Temp.V))
	}

	body := []string{Spread(pwr, t.Dim.Render(fmt.Sprintf("limit %.0fW", d.PowerLimit.V)), iw)}
	body = append(body, Graph(h.power.Values(), iw, ph, t.Power, false)...)
	body = append(body, Spread(temp, t.Dim.Render(fmt.Sprintf("slowdown %d°C", int(tempCeiling(d)))), iw))
	body = append(body, Graph(h.temp.Values(), iw, th, t.Temp, false)...)
	return Box("power", "", body, w, height, t.PwrBox, t.Title)
}

func (m Model) pcieBox(d gpu.Device, h *deviceHistory, w, height int) string {
	t := m.theme
	iw, ih := w-2, height-2
	gh := ih - 2
	rh, th := gh-gh/2, gh/2

	rx, tx := h.rx.Tail(iw*2), h.tx.Tail(iw*2)
	scale := 1.0 * (1 << 20)
	for _, v := range append(rx, tx...) {
		scale = max(scale, v)
	}
	norm := func(vs []float64) []float64 {
		out := make([]float64, len(vs))
		for i, v := range vs {
			out[i] = v / scale
		}
		return out
	}

	body := []string{Spread(t.Title.Render("▼ rx ")+t.Text.Render(Rate(d.PCIeRx.V)), t.Dim.Render("tx ")+t.Text.Render(Rate(d.PCIeTx.V))+t.Title.Render(" ▲"), iw)}
	body = append(body, Graph(norm(rx), iw, rh, t.Rx, false)...)
	body = append(body, t.Dim.Render(strings.Repeat("─", iw)))
	body = append(body, Graph(norm(tx), iw, th, t.Tx, true)...)
	footer := t.Dim.Render("scale " + Rate(uint64(scale)))
	return Box("pcie", footer, body, w, height, t.PCIeBox, t.Title)
}

func (m Model) procBox(w, height int) string {
	t := m.theme
	iw := w - 2
	const pidW, userW, typeW, pctW, memW, barW = 8, 10, 4, 5, 8, 10
	cmdW := max(iw-pidW-userW-typeW-pctW*3-memW-barW-8, 8)

	format := func(pid, user, kind, sm, enc, dec, mem, bar, cmd string) string {
		return fmt.Sprintf(" %*s %-*s %-*s%*s%*s%*s %*s %s %s",
			pidW-1, pid, userW, Fit(user, userW), typeW, kind, pctW, sm, pctW, enc, pctW, dec, memW, mem, bar, cmd)
	}

	header := t.Title.Render(Fit(format("PID", "USER", "TYPE", "SM%", "ENC", "DEC", "VRAM", strings.Repeat(" ", barW), "COMMAND"), iw))
	if m.confirm != nil {
		header = t.Warn.Render(Fit(fmt.Sprintf(" signal %s (%d)?  [t]erm  [k]ill  [any] cancel", m.confirm.Name, m.confirm.PID), iw))
	}
	body := []string{header}

	d := m.currentDevice()
	rows, first := m.procs.visible(height - 3)
	if len(rows) == 0 {
		body = append(body, t.Dim.Render(" no processes on this GPU"))
	}
	for i, p := range rows {
		frac := float64(p.GPUMem) / float64(max(d.MemTotal, 1))
		line := format(
			strconv.FormatUint(uint64(p.PID), 10), p.User, p.Kind.String(),
			pctOrDash(p.SM), pctOrDash(p.Enc), pctOrDash(p.Dec),
			Bytes(p.GPUMem), "%BAR%", Fit(p.Command, cmdW))
		before, after, _ := strings.Cut(line, "%BAR%")
		if first+i == m.procs.cursor {
			body = append(body, t.Selected.Render(Fit(before+Meter(frac, barW, plainGradient, lipgloss.NewStyle())+after, iw)))
			continue
		}
		body = append(body, t.Text.Render(before)+Meter(frac, barW, t.Mem, t.Dim)+t.Text.Render(after))
	}

	arrow := "↑"
	if m.procs.desc {
		arrow = "↓"
	}
	footer := t.Dim.Render(fmt.Sprintf("%d/%d ", min(m.procs.cursor+1, len(m.procs.rows)), len(m.procs.rows))) +
		t.Title.Render("sort:") + t.Text.Render(m.procs.sortBy.String()+" "+arrow)
	return Box("proc", footer, body, w, height, t.ProcBox, t.Title)
}

var plainGradient = NewGradient("#ffffff")

func pctOrDash(v gpu.Value[uint32]) string {
	if !v.OK {
		return "-"
	}
	return strconv.FormatUint(uint64(v.V), 10)
}

var helpKeys = [][2]string{
	{"q, ctrl+c", "quit"},
	{"?", "toggle help"},
	{"tab → l / shift+tab ← h", "next / previous GPU"},
	{"0-9", "jump to GPU"},
	{"↑ ↓ j k, pgup pgdn, g G", "move selection"},
	{"s / r", "cycle sort column / reverse"},
	{"x, del", "signal selected process"},
	{"+ / -", "slower / faster refresh"},
	{"space, p", "pause"},
	{"t", "cycle theme"},
}

func (m Model) helpView() string {
	t := m.theme
	lines := []string{""}
	for _, kv := range helpKeys {
		lines = append(lines, "  "+t.Hi.Render(fmt.Sprintf("%-26s", kv[0]))+t.Text.Render(kv[1]))
	}
	lines = append(lines, "", "  "+t.Dim.Render("theme: "+t.Name+" · press any key to close"))
	return Box("help", "", lines, 62, len(lines)+2, t.GPUBox, t.Title)
}
