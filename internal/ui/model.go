// Package ui implements the interactive terminal dashboard.
package ui

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zvdy/nvtop/internal/gpu"
	"github.com/zvdy/nvtop/internal/history"
)

const (
	historySize = 600
	minInterval = 100 * time.Millisecond
	maxInterval = 10 * time.Second
	statusTTL   = 4 * time.Second
)

// Config controls the dashboard.
type Config struct {
	Source   gpu.Source
	Interval time.Duration
	Theme    Theme
}

// Model is the Bubble Tea model for the dashboard.
type Model struct {
	src      gpu.Source
	interval time.Duration
	theme    Theme
	host     string

	snap    gpu.Snapshot
	hasSnap bool
	err     error
	hist    []*deviceHistory

	device  int
	procs   procTable
	paused  bool
	help    bool
	confirm *gpu.Process

	status      string
	statusUntil time.Time

	width, height int
}

type deviceHistory struct {
	util, mem, power, temp *history.Ring[float64]
	rx, tx                 *history.Ring[float64]
}

func newDeviceHistory() *deviceHistory {
	ring := func() *history.Ring[float64] { return history.New[float64](historySize) }
	return &deviceHistory{ring(), ring(), ring(), ring(), ring(), ring()}
}

func (h *deviceHistory) record(d gpu.Device) {
	pct := func(v gpu.Value[uint32]) float64 { return float64(v.V) / 100 }
	h.util.Push(pct(d.Util))
	h.mem.Push(d.MemFraction())
	if d.Power.OK && d.PowerLimit.OK && d.PowerLimit.V > 0 {
		h.power.Push(d.Power.V / d.PowerLimit.V)
	}
	if d.Temp.OK {
		h.temp.Push(float64(d.Temp.V) / tempCeiling(d))
	}
	h.rx.Push(float64(d.PCIeRx.V))
	h.tx.Push(float64(d.PCIeTx.V))
}

func tempCeiling(d gpu.Device) float64 {
	if d.TempSlowdown.OK && d.TempSlowdown.V > 0 {
		return float64(d.TempSlowdown.V)
	}
	return 100
}

type (
	tickMsg     struct{}
	snapshotMsg struct {
		snap gpu.Snapshot
		err  error
	}
	signalMsg struct {
		proc gpu.Process
		sig  syscall.Signal
		err  error
	}
)

// New builds a dashboard model.
func New(cfg Config) Model {
	host, _ := os.Hostname()
	return Model{
		src:      cfg.Source,
		interval: min(max(cfg.Interval, minInterval), maxInterval),
		theme:    cfg.Theme,
		host:     host,
		procs:    procTable{sortBy: sortMem, desc: true},
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return m.fetch() }

func (m Model) fetch() tea.Cmd {
	src, timeout := m.src, max(m.interval, time.Second)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		snap, err := src.Snapshot(ctx)
		return snapshotMsg{snap, err}
	}
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(m.interval, func(time.Time) tea.Msg { return tickMsg{} })
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := m.update(msg)
	m.procs.scroll(m.procRows())
	return m, cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		if m.paused {
			return m, m.tick()
		}
		return m, m.fetch()

	case snapshotMsg:
		m.err = msg.err
		if msg.err == nil {
			m.apply(msg.snap)
		}
		return m, m.tick()

	case signalMsg:
		if msg.err != nil {
			m.flash(fmt.Sprintf("%s %d: %v", msg.sig, msg.proc.PID, msg.err))
		} else {
			m.flash(fmt.Sprintf("sent %s to %s (%d)", msg.sig, msg.proc.Name, msg.proc.PID))
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) apply(snap gpu.Snapshot) {
	for len(m.hist) < len(snap.Devices) {
		m.hist = append(m.hist, newDeviceHistory())
	}
	for i, d := range snap.Devices {
		m.hist[i].record(d)
	}
	m.snap, m.hasSnap = snap, true
	m.device = min(m.device, max(len(snap.Devices)-1, 0))
	m.procs.update(m.currentDevice().Processes)
}

func (m Model) currentDevice() gpu.Device {
	if m.device < len(m.snap.Devices) {
		return m.snap.Devices[m.device]
	}
	return gpu.Device{}
}

func (m *Model) flash(s string) {
	m.status, m.statusUntil = s, time.Now().Add(statusTTL)
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	if m.confirm != nil {
		p := *m.confirm
		m.confirm = nil
		switch key {
		case "t", "y", "enter":
			return m, sendSignal(p, syscall.SIGTERM)
		case "k":
			return m, sendSignal(p, syscall.SIGKILL)
		}
		return m, nil
	}

	if m.help {
		m.help = false
		return m, nil
	}

	switch key {
	case "q":
		return m, tea.Quit
	case "?":
		m.help = true
	case " ", "p":
		m.paused = !m.paused
		if m.paused {
			m.flash("paused")
		} else {
			m.flash("resumed")
		}
	case "tab", "right", "l":
		m.selectDevice(m.device + 1)
	case "shift+tab", "left", "h":
		m.selectDevice(m.device - 1)
	case "up", "k":
		m.procs.move(-1)
	case "down", "j":
		m.procs.move(1)
	case "pgup":
		m.procs.move(-10)
	case "pgdown":
		m.procs.move(10)
	case "home", "g":
		m.procs.move(-len(m.procs.rows))
	case "end", "G":
		m.procs.move(len(m.procs.rows))
	case "s":
		m.procs.cycleSort()
	case "r":
		m.procs.desc = !m.procs.desc
		m.procs.sort()
	case "+", "=":
		m.setInterval(m.interval * 2)
	case "-", "_":
		m.setInterval(m.interval / 2)
	case "t":
		m.theme = m.theme.next()
		m.flash("theme: " + m.theme.Name)
	case "x", "delete":
		if p, ok := m.procs.selected(); ok {
			m.confirm = &p
		}
	default:
		if n := int(key[0] - '0'); len(key) == 1 && n >= 0 && n <= 9 {
			m.selectDevice(n)
		}
	}
	return m, nil
}

func (m *Model) selectDevice(i int) {
	n := len(m.snap.Devices)
	if n == 0 {
		return
	}
	m.device = ((i % n) + n) % n
	m.procs.cursor = 0
	m.procs.update(m.currentDevice().Processes)
}

func (m *Model) setInterval(d time.Duration) {
	m.interval = min(max(d, minInterval), maxInterval)
	m.flash("interval: " + m.interval.String())
}

func sendSignal(p gpu.Process, sig syscall.Signal) tea.Cmd {
	return func() tea.Msg {
		proc, err := os.FindProcess(int(p.PID))
		if err == nil {
			err = proc.Signal(sig)
		}
		return signalMsg{p, sig, err}
	}
}
