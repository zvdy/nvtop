//go:build linux && cgo

package gpu

import (
	"context"
	"errors"
	"testing"
)

func TestNVMLSnapshot(t *testing.T) {
	src, err := NewNVML()
	if errors.Is(err, ErrUnavailable) {
		t.Skipf("no NVML: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := src.Close(); err != nil {
			t.Error(err)
		}
	})

	for range 2 {
		snap, err := src.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(snap.Devices) == 0 || snap.Driver == "" {
			t.Fatalf("incomplete snapshot: %+v", snap)
		}
		for _, d := range snap.Devices {
			if d.Name == "" || d.MemTotal == 0 {
				t.Errorf("gpu%d missing static info: %+v", d.Index, d)
			}
			if d.Util.OK && d.Util.V > 100 {
				t.Errorf("gpu%d util %d > 100", d.Index, d.Util.V)
			}
		}
	}
}
