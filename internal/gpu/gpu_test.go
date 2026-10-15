package gpu

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValueJSON(t *testing.T) {
	type payload struct {
		A Value[uint32] `json:"a"`
		B Value[uint32] `json:"b"`
	}
	b, err := json.Marshal(payload{A: Some[uint32](7)})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), `{"a":7,"b":null}`; got != want {
		t.Fatalf("Marshal = %s, want %s", got, want)
	}

	var p payload
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if !p.A.OK || p.A.V != 7 || p.B.OK {
		t.Fatalf("round trip = %+v", p)
	}
}

func TestProcKind(t *testing.T) {
	tests := map[ProcKind]string{
		KindCompute:                "C",
		KindGraphics:               "G",
		KindCompute | KindGraphics: "C+G",
		0:                          "?",
	}
	for k, want := range tests {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", k, got, want)
		}
	}
}

func TestMemFraction(t *testing.T) {
	if f := (Device{}).MemFraction(); f != 0 {
		t.Errorf("zero total: got %v", f)
	}
	if f := (Device{MemUsed: 1, MemTotal: 4}).MemFraction(); f != 0.25 {
		t.Errorf("got %v, want 0.25", f)
	}
}

func TestMockSnapshot(t *testing.T) {
	m := NewMock(3, 1)
	snap, err := m.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Devices) != 3 {
		t.Fatalf("got %d devices, want 3", len(snap.Devices))
	}
	for _, d := range snap.Devices {
		if !d.Util.OK || d.Util.V > 100 {
			t.Errorf("gpu%d util out of range: %+v", d.Index, d.Util)
		}
		if d.MemUsed > d.MemTotal {
			t.Errorf("gpu%d used %d > total %d", d.Index, d.MemUsed, d.MemTotal)
		}
		if len(d.Processes) == 0 {
			t.Errorf("gpu%d has no processes", d.Index)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Snapshot(ctx); err == nil {
		t.Error("expected error for canceled context")
	}
}

func TestProcCache(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "1234")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, data string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("comm", "python3\n")
	write("cmdline", "python3\x00train.py\x00--epochs\x0010\x00")
	write("status", "Name:\tpython3\nUid:\t4242\t4242\t4242\t4242\n")

	c := newProcCache()
	c.root = root
	c.users["4242"] = "alice"

	info := c.lookup(1234)
	want := procInfo{name: "python3", user: "alice", command: "python3 train.py --epochs 10"}
	if info != want {
		t.Fatalf("lookup = %+v, want %+v", info, want)
	}

	if missing := c.lookup(99999); missing.name != "?" {
		t.Errorf("missing process name = %q, want ?", missing.name)
	}

	c.prune()
	c.prune()
	if len(c.procs) != 0 {
		t.Errorf("prune kept %d stale entries", len(c.procs))
	}
}
