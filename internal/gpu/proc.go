package gpu

import (
	"bytes"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type procInfo struct {
	name    string
	user    string
	command string
}

// procCache memoises /proc lookups; entries not touched since the previous
// prune are dropped so recycled PIDs are re-resolved.
type procCache struct {
	mu    sync.Mutex
	root  string
	procs map[uint32]procInfo
	seen  map[uint32]bool
	users map[string]string
}

func newProcCache() *procCache {
	return &procCache{
		root:  "/proc",
		procs: map[uint32]procInfo{},
		seen:  map[uint32]bool{},
		users: map[string]string{},
	}
}

func (c *procCache) lookup(pid uint32) procInfo {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.seen[pid] = true
	if info, ok := c.procs[pid]; ok {
		return info
	}
	info := c.read(pid)
	c.procs[pid] = info
	return info
}

func (c *procCache) prune() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for pid := range c.procs {
		if !c.seen[pid] {
			delete(c.procs, pid)
		}
	}
	clear(c.seen)
}

func (c *procCache) read(pid uint32) procInfo {
	dir := filepath.Join(c.root, strconv.FormatUint(uint64(pid), 10))
	info := procInfo{name: "?", user: "?"}

	if b, err := os.ReadFile(filepath.Join(dir, "comm")); err == nil {
		info.name = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil {
		info.command = strings.TrimSpace(string(bytes.ReplaceAll(b, []byte{0}, []byte{' '})))
	}
	if info.command == "" {
		info.command = info.name
	}
	if b, err := os.ReadFile(filepath.Join(dir, "status")); err == nil {
		info.user = c.username(parseUID(b))
	}
	return info
}

func (c *procCache) username(uid string) string {
	if uid == "" {
		return "?"
	}
	if name, ok := c.users[uid]; ok {
		return name
	}
	name := uid
	if u, err := user.LookupId(uid); err == nil {
		name = u.Username
	}
	c.users[uid] = name
	return name
}

func parseUID(status []byte) string {
	for line := range bytes.Lines(status) {
		rest, ok := bytes.CutPrefix(line, []byte("Uid:"))
		if !ok {
			continue
		}
		if f := strings.Fields(string(rest)); len(f) > 0 {
			return f[0]
		}
	}
	return ""
}
