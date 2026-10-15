package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/zvdy/nvtop/internal/gpu"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    options
		wantErr string
	}{
		{name: "defaults", want: options{interval: time.Second, theme: "default"}},
		{name: "short interval", args: []string{"-i", "250ms"}, want: options{interval: 250 * time.Millisecond, theme: "default"}},
		{name: "mock and theme", args: []string{"-mock", "4", "-theme", "nord"}, want: options{interval: time.Second, theme: "nord", mock: 4}},
		{name: "negative interval", args: []string{"-i", "-1s"}, wantErr: "interval must be positive"},
		{name: "negative mock", args: []string{"-mock", "-1"}, wantErr: "non-negative"},
		{name: "positional", args: []string{"foo"}, wantErr: "unexpected arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFlags(tt.args, io.Discard)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRunJSON(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-mock", "2", "-json"}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	var snap gpu.Snapshot
	if err := json.Unmarshal(out.Bytes(), &snap); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(snap.Devices) != 2 {
		t.Fatalf("got %d devices, want 2", len(snap.Devices))
	}
}

func TestRunVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-version"}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "nvtop ") {
		t.Fatalf("unexpected version output %q", out.String())
	}
}

func TestRunUnknownTheme(t *testing.T) {
	err := run([]string{"-mock", "1", "-theme", "nope"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "unknown theme") {
		t.Fatalf("err = %v, want unknown theme", err)
	}
}
