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
	if cfg == nil {
		return rot
	}

	authDir := strings.TrimSpace(cfg.AuthDir)
	if authDir != "" {
		authDir = expandPath(authDir)
	}

	seen := make(map[string]struct{})
	addEntry := func(path, region, label string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if !filepath.IsAbs(path) && authDir != "" {
			path = filepath.Join(authDir, path)
		}
		if !filepath.IsAbs(path) {
			return
		}
		path = filepath.Clean(path)
		if _, exists := seen[path]; exists {
			return
		}
		seen[path] = struct{}{}
		rot.entries = append(rot.entries, kiroRotatorEntry{
			path:   path,
			region: strings.TrimSpace(region),
			label:  strings.TrimSpace(label),
		})
	}

	for _, entry := range cfg.KiroTokenFiles {
		path := entry.TokenFilePath
		addEntry(path, entry.Region, entry.Label)
	}

	if authDir != "" {
		for _, path := range discoverKiroTokenFiles(authDir) {
			addEntry(path, "", filepath.Base(path))
		}
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
