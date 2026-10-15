package ui

import (
	"cmp"
	"slices"

	"github.com/zvdy/nvtop/internal/gpu"
)

type sortKey int

const (
	sortMem sortKey = iota
	sortSM
	sortPID
	sortName
	sortUser
	numSortKeys
)

func (k sortKey) String() string {
	return [...]string{"vram", "sm", "pid", "name", "user"}[k]
}

// procTable keeps process rows sorted and the selection stable across
// refreshes by tracking the selected PID rather than an index.
type procTable struct {
	rows   []gpu.Process
	cursor int
	offset int
	selPID uint32
	sortBy sortKey
	desc   bool
}

func (t *procTable) update(rows []gpu.Process) {
	t.rows = slices.Clone(rows)
	t.sort()
}

func (t *procTable) sort() {
	slices.SortStableFunc(t.rows, func(a, b gpu.Process) int {
		var c int
		switch t.sortBy {
		case sortMem:
			c = cmp.Compare(a.GPUMem, b.GPUMem)
		case sortSM:
			c = cmp.Compare(a.SM.V, b.SM.V)
		case sortName:
			c = cmp.Compare(a.Name, b.Name)
		case sortUser:
			c = cmp.Compare(a.User, b.User)
		}
		if c == 0 {
			c = cmp.Compare(a.PID, b.PID)
		}
		if t.desc {
			return -c
		}
		return c
	})

	if i := slices.IndexFunc(t.rows, func(p gpu.Process) bool { return p.PID == t.selPID }); i >= 0 {
		t.cursor = i
	}
	t.clamp()
}

func (t *procTable) cycleSort() {
	t.sortBy = (t.sortBy + 1) % numSortKeys
	t.desc = t.sortBy == sortMem || t.sortBy == sortSM
	t.sort()
}

func (t *procTable) move(delta int) {
	t.cursor += delta
	t.clamp()
}

func (t *procTable) clamp() {
	t.cursor = min(max(t.cursor, 0), max(len(t.rows)-1, 0))
	if len(t.rows) > 0 {
		t.selPID = t.rows[t.cursor].PID
	}
}

func (t *procTable) selected() (gpu.Process, bool) {
	if t.cursor < len(t.rows) {
		return t.rows[t.cursor], true
	}
	return gpu.Process{}, false
}

// scroll adjusts the offset just enough to keep the cursor within a
// viewport of the given height.
func (t *procTable) scroll(height int) {
	if height <= 0 {
		return
	}
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+height {
		t.offset = t.cursor - height + 1
	}
	t.offset = max(min(t.offset, len(t.rows)-height), 0)
}

// visible returns the rows in the viewport and the index of the first one.
func (t procTable) visible(height int) ([]gpu.Process, int) {
	if height <= 0 || t.offset >= len(t.rows) {
		return nil, 0
	}
	return t.rows[t.offset:min(t.offset+height, len(t.rows))], t.offset
}
