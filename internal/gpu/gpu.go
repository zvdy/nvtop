// Package gpu defines a backend-agnostic view of NVIDIA GPU telemetry.
package gpu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrUnavailable is returned when no usable GPU backend can be initialized.
var ErrUnavailable = errors.New("gpu backend unavailable")

// Source produces point-in-time telemetry snapshots.
type Source interface {
	Snapshot(ctx context.Context) (Snapshot, error)
	Close() error
}

// Value is a metric that a device may not support.
type Value[T any] struct {
	V  T
	OK bool
}

// Some wraps a supported value.
func Some[T any](v T) Value[T] { return Value[T]{V: v, OK: true} }

// MarshalJSON encodes unsupported values as null.
func (v Value[T]) MarshalJSON() ([]byte, error) {
	if !v.OK {
		return []byte("null"), nil
	}
	return json.Marshal(v.V)
}

// UnmarshalJSON is the inverse of MarshalJSON.
func (v *Value[T]) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*v = Value[T]{}
		return nil
	}
	v.OK = true
	return json.Unmarshal(b, &v.V)
}

// Snapshot is the state of every device at a single instant.
type Snapshot struct {
	Taken   time.Time `json:"taken"`
	Driver  string    `json:"driver"`
	CUDA    string    `json:"cuda"`
	Devices []Device  `json:"devices"`
}

// Device holds the telemetry of a single GPU. Percentages are 0-100,
// power is in watts, clocks in MHz and throughput in bytes per second.
type Device struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	UUID  string `json:"uuid"`
	BusID string `json:"bus_id"`

	Util    Value[uint32] `json:"util"`
	MemUtil Value[uint32] `json:"mem_util"`
	EncUtil Value[uint32] `json:"enc_util"`
	DecUtil Value[uint32] `json:"dec_util"`

	MemUsed  uint64 `json:"mem_used"`
	MemTotal uint64 `json:"mem_total"`

	Temp         Value[uint32]  `json:"temp"`
	TempSlowdown Value[uint32]  `json:"temp_slowdown"`
	Power        Value[float64] `json:"power"`
	PowerLimit   Value[float64] `json:"power_limit"`
	Fan          Value[uint32]  `json:"fan"`

	ClockGfx    Value[uint32] `json:"clock_gfx"`
	ClockGfxMax Value[uint32] `json:"clock_gfx_max"`
	ClockMem    Value[uint32] `json:"clock_mem"`
	ClockMemMax Value[uint32] `json:"clock_mem_max"`

	PCIeRx Value[uint64] `json:"pcie_rx"`
	PCIeTx Value[uint64] `json:"pcie_tx"`
	PState Value[int]    `json:"pstate"`

	Processes []Process `json:"processes"`
}

// MemFraction returns used/total VRAM in the range [0,1].
func (d Device) MemFraction() float64 {
	if d.MemTotal == 0 {
		return 0
	}
	return float64(d.MemUsed) / float64(d.MemTotal)
}

// ProcKind describes how a process is using the GPU.
type ProcKind uint8

const (
	// KindCompute marks a CUDA/compute context.
	KindCompute ProcKind = 1 << iota
	// KindGraphics marks a graphics (OpenGL/Vulkan/display) context.
	KindGraphics
)

func (k ProcKind) String() string {
	switch k {
	case KindCompute:
		return "C"
	case KindGraphics:
		return "G"
	case KindCompute | KindGraphics:
		return "C+G"
	default:
		return "?"
	}
}

// MarshalText renders the kind in its short form for JSON output.
func (k ProcKind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// UnmarshalText is the inverse of MarshalText.
func (k *ProcKind) UnmarshalText(b []byte) error {
	switch string(b) {
	case "C":
		*k = KindCompute
	case "G":
		*k = KindGraphics
	case "C+G":
		*k = KindCompute | KindGraphics
	case "?":
		*k = 0
	default:
		return fmt.Errorf("invalid process kind %q", b)
	}
	return nil
}

// Process is a process holding a GPU context.
type Process struct {
	PID     uint32        `json:"pid"`
	Name    string        `json:"name"`
	User    string        `json:"user"`
	Command string        `json:"command"`
	Kind    ProcKind      `json:"kind"`
	GPUMem  uint64        `json:"gpu_mem"`
	SM      Value[uint32] `json:"sm"`
	Enc     Value[uint32] `json:"enc"`
	Dec     Value[uint32] `json:"dec"`
}
