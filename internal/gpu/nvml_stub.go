//go:build !(linux && cgo)

package gpu

import (
	"context"
	"fmt"
)

// NVML is unavailable on this platform or without cgo.
type NVML struct{}

// NewNVML always fails on unsupported builds.
func NewNVML() (*NVML, error) {
	return nil, fmt.Errorf("%w: built without linux+cgo support", ErrUnavailable)
}

// Snapshot implements Source.
func (*NVML) Snapshot(context.Context) (Snapshot, error) { return Snapshot{}, ErrUnavailable }

// Close implements Source.
func (*NVML) Close() error { return nil }
