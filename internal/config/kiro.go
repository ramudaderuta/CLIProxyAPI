package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
)

const defaultKiroRegion = "us-east-1"

// KiroTokenFile represents a single Kiro token file configuration entry.
type KiroTokenFile struct {
	TokenFilePath string `yaml:"token-file-path" json:"token-file-path"`
	Region        string `yaml:"region,omitempty" json:"region,omitempty"`
	Label         string `yaml:"label,omitempty" json:"label,omitempty"`
}

func (k *KiroTokenFile) normalize() {
	if k == nil {
		return
	}
	k.TokenFilePath = strings.TrimSpace(k.TokenFilePath)
	k.Region = strings.TrimSpace(k.Region)
	if k.Region == "" {
		k.Region = defaultKiroRegion
	}
	k.Label = strings.TrimSpace(k.Label)
}

// ResolvePath returns an absolute path to the configured token file.
func (k KiroTokenFile) ResolvePath(authDir string) (string, error) {
	path := strings.TrimSpace(k.TokenFilePath)
	if path == "" {
		return "", fmt.Errorf("token-file-path is required")
	}
	if expanded, err := expandUserPath(path); err == nil && expanded != "" {
		path = expanded
	} else if err != nil {
		return "", err
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	if strings.TrimSpace(authDir) == "" {
		return "", fmt.Errorf("token-file-path %q is relative but auth-dir is not configured", k.TokenFilePath)
	}
	base, err := expandUserPath(authDir)
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(base, path)), nil
}

// NormalizeKiroTokenFiles deduplicates and normalizes Kiro token configuration entries.
func (cfg *Config) NormalizeKiroTokenFiles() {
	if cfg == nil || len(cfg.KiroTokenFiles) == 0 {
		return
	}
	unique := make(map[string]struct{}, len(cfg.KiroTokenFiles))
	out := cfg.KiroTokenFiles[:0]
	for i := range cfg.KiroTokenFiles {
		entry := cfg.KiroTokenFiles[i]
		entry.normalize()
		if entry.TokenFilePath == "" {
			continue
		}
		key := strings.ToLower(entry.TokenFilePath + "|" + entry.Region)
		if _, exists := unique[key]; exists {
			continue
		}
		unique[key] = struct{}{}
		out = append(out, entry)
	}
	cfg.KiroTokenFiles = out
}

// ValidateKiroTokenFiles ensures configured token files exist and contain required fields.
func (cfg *Config) ValidateKiroTokenFiles() error {
	if cfg == nil {
		return nil
	}
	if len(cfg.KiroTokenFiles) == 0 {
		return nil
	}

	authDir := strings.TrimSpace(cfg.AuthDir)
	if resolved, err := expandUserPath(authDir); err == nil && resolved != "" {
		authDir = resolved
	} else if err != nil {
		return fmt.Errorf("invalid auth-dir for kiro configuration: %w", err)
	}

	seen := make(map[string]struct{}, len(cfg.KiroTokenFiles))
	for idx := range cfg.KiroTokenFiles {
		entry := cfg.KiroTokenFiles[idx]
		resolvedPath, err := entry.ResolvePath(authDir)
		if err != nil {
			return fmt.Errorf("kiro token file[%d]: %w", idx, err)
		}
		if _, exists := seen[resolvedPath]; exists {
			continue
		}
		seen[resolvedPath] = struct{}{}

		info, err := os.Stat(resolvedPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("kiro token file[%d]: %s does not exist", idx, resolvedPath)
			}
			return fmt.Errorf("kiro token file[%d]: failed to stat %s: %w", idx, resolvedPath, err)
		}
		if info.IsDir() {
			return fmt.Errorf("kiro token file[%d]: %s is a directory", idx, resolvedPath)
		}

		token, err := kiro.LoadTokenFromFile(resolvedPath)
		if err != nil {
			return fmt.Errorf("kiro token file[%d]: %w", idx, err)
		}
		if err := ensureKiroRequiredFields(token); err != nil {
			return fmt.Errorf("kiro token file[%d]: %w", idx, err)
		}
	}
	return nil
}

func ensureKiroRequiredFields(token *kiro.KiroTokenStorage) error {
	if token == nil {
		return fmt.Errorf("token payload is empty")
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return fmt.Errorf("accessToken is required")
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		return fmt.Errorf("refreshToken is required")
	}
	if token.ExpiresAt.IsZero() {
		return fmt.Errorf("expiresAt is required")
	}
	return nil
}

func expandUserPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path[0] != '~' {
		return filepath.Clean(path), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if path == "~" {
		return filepath.Clean(home), nil
	}
	remainder := strings.TrimLeft(path[1:], string(filepath.Separator))
	remainder = strings.TrimLeft(remainder, "/\\")
	if remainder == "" {
		return filepath.Clean(home), nil
	}
	return filepath.Clean(filepath.Join(home, remainder)), nil
}
