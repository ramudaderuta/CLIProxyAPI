package executor

import (
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

type kiroTokenRotator struct {
	entries []kiroRotatorEntry
	cursor  uint32
}

type kiroRotatorEntry struct {
	path   string
	region string
	label  string
}

func newKiroTokenRotator(cfg *config.Config) *kiroTokenRotator {
	rot := &kiroTokenRotator{}
	if cfg == nil || len(cfg.KiroTokenFiles) == 0 {
		return rot
	}

	authDir := strings.TrimSpace(cfg.AuthDir)
	if authDir != "" {
		authDir = expandPath(authDir)
	}

	for _, entry := range cfg.KiroTokenFiles {
		path := strings.TrimSpace(entry.TokenFilePath)
		if path == "" {
			continue
		}
		path = expandPath(path)
		if !filepath.IsAbs(path) && authDir != "" {
			path = filepath.Join(authDir, path)
		}
		if path == "" {
			continue
		}
		rot.entries = append(rot.entries, kiroRotatorEntry{
			path:   filepath.Clean(path),
			region: entry.Region,
			label:  entry.Label,
		})
	}

	return rot
}

func (r *kiroTokenRotator) count() int {
	if r == nil {
		return 0
	}
	return len(r.entries)
}

func (r *kiroTokenRotator) candidates() []kiroTokenCandidate {
	if r == nil || len(r.entries) == 0 {
		return nil
	}
	length := len(r.entries)
	start := 0
	if length > 0 {
		start = int(atomic.LoadUint32(&r.cursor) % uint32(length))
	}

	candidates := make([]kiroTokenCandidate, 0, length)
	for offset := 0; offset < length; offset++ {
		idx := (start + offset) % length
		entry := r.entries[idx]
		candidates = append(candidates, kiroTokenCandidate{
			path:         entry.path,
			region:       entry.region,
			label:        entry.label,
			fromRotator:  true,
			rotatorIndex: idx,
		})
	}
	return candidates
}

func (r *kiroTokenRotator) advance(idx int) {
	if r == nil || len(r.entries) == 0 {
		return
	}
	next := (idx + 1) % len(r.entries)
	atomic.StoreUint32(&r.cursor, uint32(next))
}
