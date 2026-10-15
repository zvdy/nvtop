package gpu

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"time"
)

// Mock simulates GPUs with plausible, smoothly varying load. It is useful for
// demos, screenshots and development on machines without NVIDIA hardware.
type Mock struct {
	mu    sync.Mutex
	rng   *rand.Rand
	start time.Time
	now   func() time.Time
	gpus  []mockGPU
}

type mockGPU struct {
	name   string
	memGiB uint64
	tdp    float64
	phase  float64
	procs  []Process
}

var mockModels = []struct {
	name   string
	memGiB uint64
	tdp    float64
}{
	{"NVIDIA GeForce RTX 4090", 24, 450},
	{"NVIDIA H100 80GB HBM3", 80, 700},
	{"NVIDIA RTX A6000", 48, 300},
	{"NVIDIA GeForce RTX 3080", 10, 320},
}

var mockProcs = []struct{ name, cmd string }{
	{"python3", "python3 train.py --model llama-8b --bf16"},
	{"ollama", "ollama serve"},
	{"Xorg", "/usr/lib/Xorg :0 -seat seat0"},
	{"blender", "blender -b scene.blend -a"},
	{"ffmpeg", "ffmpeg -hwaccel cuda -i in.mkv -c:v h264_nvenc out.mp4"},
	{"firefox", "/usr/lib/firefox/firefox"},
	{"vllm", "python -m vllm.entrypoints.openai.api_server"},
}

// NewMock returns a simulated source with n devices. A fixed seed yields a
// reproducible sequence.
func NewMock(n int, seed uint64) *Mock {
	m := &Mock{
		rng:   rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
		start: time.Now(),
		now:   time.Now,
	}
	for i := range max(n, 1) {
		model := mockModels[i%len(mockModels)]
		g := mockGPU{name: model.name, memGiB: model.memGiB, tdp: model.tdp, phase: m.rng.Float64() * math.Pi * 2}
		for j := range 2 + m.rng.IntN(4) {
			p := mockProcs[(i*3+j)%len(mockProcs)]
			kind := KindCompute
			if p.name == "Xorg" || p.name == "firefox" || p.name == "blender" {
				kind = KindGraphics
			}
			g.procs = append(g.procs, Process{
				PID:     uint32(1000 + m.rng.IntN(60000)),
				Name:    p.name,
				User:    "demo",
				Command: p.cmd,
				Kind:    kind,
				GPUMem:  uint64(m.rng.IntN(int(model.memGiB*1024)/4)+64) << 20,
			})
		}
		m.gpus = append(m.gpus, g)
	}
	return m
}

// Close implements Source.
func (m *Mock) Close() error { return nil }

// Snapshot implements Source.
func (m *Mock) Snapshot(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	t := now.Sub(m.start).Seconds()
	snap := Snapshot{Taken: now, Driver: "580.65.06", CUDA: "13.0"}

	for i, g := range m.gpus {
		load := clamp01(0.55 + 0.35*math.Sin(t/7+g.phase) + 0.15*math.Sin(t/1.3+g.phase*2) + m.jitter(0.08))
		util := uint32(load * 100)
		total := g.memGiB << 30

		var used uint64
		procs := make([]Process, len(g.procs))
		for j, p := range g.procs {
			p.SM = Some(uint32(clamp01(load*float64(len(g.procs)-j)/float64(len(g.procs))+m.jitter(0.05)) * 100))
			if p.Name == "ffmpeg" {
				p.Enc = Some(uint32(40 + m.rng.IntN(30)))
				p.Dec = Some(uint32(20 + m.rng.IntN(20)))
			}
			procs[j] = p
			used += p.GPUMem
		}
		used = min(used+uint64(load*float64(total)*0.2), total)

		temp := 38 + load*44 + m.jitter(1.5)
		power := g.tdp*0.12 + g.tdp*0.8*load + m.jitter(4)

		snap.Devices = append(snap.Devices, Device{
			Index:        i,
			Name:         g.name,
			UUID:         fmt.Sprintf("GPU-%08x-mock-%04d", 0xdeadbeef+i, i),
			BusID:        fmt.Sprintf("00000000:%02X:00.0", 0x01+i*0x20),
			Util:         Some(util),
			MemUtil:      Some(uint32(load * 70)),
			EncUtil:      Some(uint32(clamp01(0.3+0.3*math.Sin(t/3)) * 100)),
			DecUtil:      Some(uint32(clamp01(0.15+0.2*math.Cos(t/4)) * 100)),
			MemUsed:      used,
			MemTotal:     total,
			Temp:         Some(uint32(temp)),
			TempSlowdown: Some(uint32(90)),
			Power:        Some(power),
			PowerLimit:   Some(g.tdp),
			Fan:          Some(uint32(30 + load*55)),
			ClockGfx:     Some(uint32(210 + load*2310)),
			ClockGfxMax:  Some(uint32(2520)),
			ClockMem:     Some(uint32(10501)),
			ClockMemMax:  Some(uint32(10501)),
			PCIeRx:       Some(uint64(clamp01(load+m.jitter(0.3)) * 12e9)),
			PCIeTx:       Some(uint64(clamp01(load*0.4+m.jitter(0.2)) * 6e9)),
			PState:       Some(pstate(load)),
			Processes:    procs,
		})
	}
	return snap, nil
}

func (m *Mock) jitter(scale float64) float64 { return (m.rng.Float64()*2 - 1) * scale }

func pstate(load float64) int {
	if load > 0.2 {
		return 0
	}
	return 8
}

func clamp01(v float64) float64 { return min(max(v, 0), 1) }
