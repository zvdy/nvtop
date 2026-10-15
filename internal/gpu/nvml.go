//go:build linux && cgo

package gpu

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// NVML reads telemetry through the NVIDIA Management Library.
type NVML struct {
	mu       sync.Mutex
	devices  []nvml.Device
	static   []Device
	driver   string
	cuda     string
	lastSeen []uint64
	procs    *procCache
}

// NewNVML initializes NVML and enumerates devices.
func NewNVML() (*NVML, error) {
	if ret := nvml.Init(); ret != nvml.SUCCESS {
		return nil, fmt.Errorf("%w: nvml init: %s", ErrUnavailable, nvml.ErrorString(ret))
	}

	n := &NVML{procs: newProcCache()}
	count, ret := nvml.DeviceGetCount()
	if err := check("device count", ret); err != nil {
		_ = nvml.Shutdown()
		return nil, err
	}
	if count == 0 {
		_ = nvml.Shutdown()
		return nil, fmt.Errorf("%w: no NVIDIA devices found", ErrUnavailable)
	}

	if v, ret := nvml.SystemGetDriverVersion(); ret == nvml.SUCCESS {
		n.driver = v
	}
	if v, ret := nvml.SystemGetCudaDriverVersion(); ret == nvml.SUCCESS {
		n.cuda = fmt.Sprintf("%d.%d", v/1000, v%1000/10)
	}

	for i := range count {
		dev, ret := nvml.DeviceGetHandleByIndex(i)
		if err := check(fmt.Sprintf("device %d handle", i), ret); err != nil {
			_ = nvml.Shutdown()
			return nil, err
		}
		n.devices = append(n.devices, dev)
		n.static = append(n.static, staticInfo(i, dev))
	}
	n.lastSeen = make([]uint64, count)
	return n, nil
}

// Close releases NVML.
func (n *NVML) Close() error {
	return check("shutdown", nvml.Shutdown())
}

// Snapshot samples every device.
func (n *NVML) Snapshot(ctx context.Context) (Snapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	snap := Snapshot{
		Taken:   time.Now(),
		Driver:  n.driver,
		CUDA:    n.cuda,
		Devices: make([]Device, len(n.devices)),
	}
	for i, dev := range n.devices {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		d := n.static[i]
		sampleDynamic(&d, dev)
		d.Processes = n.processes(i, dev)
		snap.Devices[i] = d
	}
	n.procs.prune()
	return snap, nil
}

func staticInfo(i int, dev nvml.Device) Device {
	d := Device{Index: i}
	if v, ret := dev.GetName(); ret == nvml.SUCCESS {
		d.Name = v
	}
	if v, ret := dev.GetUUID(); ret == nvml.SUCCESS {
		d.UUID = v
	}
	if pci, ret := dev.GetPciInfo(); ret == nvml.SUCCESS {
		d.BusID = cString(pci.BusIdLegacy[:])
	}
	d.TempSlowdown = opt(dev.GetTemperatureThreshold(nvml.TEMPERATURE_THRESHOLD_SLOWDOWN))
	d.ClockGfxMax = opt(dev.GetMaxClockInfo(nvml.CLOCK_GRAPHICS))
	d.ClockMemMax = opt(dev.GetMaxClockInfo(nvml.CLOCK_MEM))
	return d
}

func sampleDynamic(d *Device, dev nvml.Device) {
	if u, ret := dev.GetUtilizationRates(); ret == nvml.SUCCESS {
		d.Util, d.MemUtil = Some(u.Gpu), Some(u.Memory)
	}
	if v, _, ret := dev.GetEncoderUtilization(); ret == nvml.SUCCESS {
		d.EncUtil = Some(v)
	}
	if v, _, ret := dev.GetDecoderUtilization(); ret == nvml.SUCCESS {
		d.DecUtil = Some(v)
	}
	if m, ret := dev.GetMemoryInfo(); ret == nvml.SUCCESS {
		d.MemUsed, d.MemTotal = m.Used, m.Total
	}

	d.Temp = opt(dev.GetTemperature(nvml.TEMPERATURE_GPU))
	d.Fan = opt(dev.GetFanSpeed())
	d.ClockGfx = opt(dev.GetClockInfo(nvml.CLOCK_GRAPHICS))
	d.ClockMem = opt(dev.GetClockInfo(nvml.CLOCK_MEM))

	if mw, ret := dev.GetPowerUsage(); ret == nvml.SUCCESS {
		d.Power = Some(float64(mw) / 1000)
	}
	if mw, ret := dev.GetEnforcedPowerLimit(); ret == nvml.SUCCESS {
		d.PowerLimit = Some(float64(mw) / 1000)
	}
	if kb, ret := dev.GetPcieThroughput(nvml.PCIE_UTIL_RX_BYTES); ret == nvml.SUCCESS {
		d.PCIeRx = Some(uint64(kb) * 1024)
	}
	if kb, ret := dev.GetPcieThroughput(nvml.PCIE_UTIL_TX_BYTES); ret == nvml.SUCCESS {
		d.PCIeTx = Some(uint64(kb) * 1024)
	}
	if p, ret := dev.GetPerformanceState(); ret == nvml.SUCCESS && p != nvml.PSTATE_UNKNOWN {
		d.PState = Some(int(p))
	}
}

func (n *NVML) processes(i int, dev nvml.Device) []Process {
	byPID := map[uint32]*Process{}
	collect := func(infos []nvml.ProcessInfo, ret nvml.Return, kind ProcKind) {
		if ret != nvml.SUCCESS {
			return
		}
		for _, info := range infos {
			p, ok := byPID[info.Pid]
			if !ok {
				p = &Process{PID: info.Pid}
				byPID[info.Pid] = p
			}
			p.Kind |= kind
			// UsedGpuMemory is NVML_VALUE_NOT_AVAILABLE (max uint64) without privileges.
			if info.UsedGpuMemory != ^uint64(0) {
				p.GPUMem = max(p.GPUMem, info.UsedGpuMemory)
			}
		}
	}
	compute, ret := dev.GetComputeRunningProcesses()
	collect(compute, ret, KindCompute)
	graphics, ret := dev.GetGraphicsRunningProcesses()
	collect(graphics, ret, KindGraphics)
	if len(byPID) == 0 {
		return nil
	}

	since := n.lastSeen[i]
	if since == 0 {
		since = uint64(time.Now().Add(-time.Second).UnixMicro())
	}
	if samples, ret := dev.GetProcessUtilization(since); ret == nvml.SUCCESS {
		latest := map[uint32]uint64{}
		for _, s := range samples {
			p, ok := byPID[s.Pid]
			if !ok || s.TimeStamp < latest[s.Pid] {
				continue
			}
			latest[s.Pid] = s.TimeStamp
			p.SM, p.Enc, p.Dec = Some(s.SmUtil), Some(s.EncUtil), Some(s.DecUtil)
			n.lastSeen[i] = max(n.lastSeen[i], s.TimeStamp)
		}
	}

	out := make([]Process, 0, len(byPID))
	for _, p := range byPID {
		info := n.procs.lookup(p.PID)
		p.Name, p.User, p.Command = info.name, info.user, info.command
		out = append(out, *p)
	}
	slices.SortFunc(out, func(a, b Process) int { return cmp.Compare(a.PID, b.PID) })
	return out
}

func opt(v uint32, ret nvml.Return) Value[uint32] {
	if ret != nvml.SUCCESS {
		return Value[uint32]{}
	}
	return Some(v)
}

func check(op string, ret nvml.Return) error {
	if ret == nvml.SUCCESS {
		return nil
	}
	return fmt.Errorf("nvml %s: %w", op, errors.New(nvml.ErrorString(ret)))
}

func cString[T ~int8 | ~uint8](b []T) string {
	var sb strings.Builder
	for _, c := range b {
		if c == 0 {
			break
		}
		sb.WriteByte(byte(c))
	}
	return sb.String()
}
