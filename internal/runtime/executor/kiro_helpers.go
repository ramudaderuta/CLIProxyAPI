package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

const kiroTokenPathMetadataKey = "_kiro_token_path"

type kiroTokenCandidate struct {
	path         string
	region       string
	label        string
	fromRotator  bool
	rotatorIndex int
}

func (e *KiroExecutor) tokenStorageFromAuth(ctx context.Context, auth *cliproxyauth.Auth) (*authkiro.KiroTokenStorage, error) {
	if auth == nil {
		return nil, fmt.Errorf("kiro executor: auth is nil")
	}

	rotate := e.shouldRotateConfiguredTokens(auth)

	if !rotate {
		if ts, ok := auth.Runtime.(*authkiro.KiroTokenStorage); ok && ts != nil {
			if err := e.client.ensureToken(ctx, ts); err == nil {
				return ts, nil
			}
			auth.Runtime = nil
		}
		if token := extractTokenFromMetadata(auth.Metadata); token != nil {
			if err := e.client.ensureToken(ctx, token); err == nil {
				auth.Runtime = token
				return token, nil
			}
		}
	}

	candidates := e.tokenFileCandidates(auth, rotate)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("kiro executor: token path unavailable for %s", auth.ID)
	}

	var errs []error
	for _, cand := range candidates {
		ts, err := authkiro.LoadTokenFromFile(cand.path)
		if err != nil {
			errs = append(errs, fmt.Errorf("load %s: %w", cand.describe(), err))
			e.advanceRotatorAfterAttempt(cand)
			continue
		}
		if err := e.client.ensureToken(ctx, ts); err != nil {
			errs = append(errs, fmt.Errorf("refresh %s: %w", cand.describe(), err))
			e.advanceRotatorAfterAttempt(cand)
			continue
		}
		e.applyCandidateSelection(auth, ts, cand)
		e.advanceRotatorAfterAttempt(cand)
		return ts, nil
	}

	if len(errs) == 1 {
		return nil, errs[0]
	}
	return nil, errors.Join(errs...)
}

func (e *KiroExecutor) shouldRotateConfiguredTokens(auth *cliproxyauth.Auth) bool {
	if e == nil || e.tokenRotator == nil {
		return false
	}
	if e.tokenRotator.count() < 2 {
		return false
	}
	if auth == nil {
		return true
	}
	return strings.TrimSpace(e.attributeTokenPath(auth)) == ""
}

func (e *KiroExecutor) tokenFileCandidates(auth *cliproxyauth.Auth, rotate bool) []kiroTokenCandidate {
	if auth == nil {
		return nil
	}

	if path := e.attributeTokenPath(auth); path != "" {
		return []kiroTokenCandidate{{path: path}}
	}

	rotatorCandidates := e.rotatorCandidates()
	if rotate && len(rotatorCandidates) > 0 {
		return rotatorCandidates
	}

	if !rotate {
		if path := e.metadataTokenPath(auth); path != "" {
			return []kiroTokenCandidate{{path: path}}
		}
		if len(rotatorCandidates) > 0 {
			return rotatorCandidates
		}
	}

	return e.fallbackTokenCandidates(auth)
}

func (e *KiroExecutor) attributeTokenPath(auth *cliproxyauth.Auth) string {
	if auth == nil || auth.Attributes == nil {
		return ""
	}
	if p := strings.TrimSpace(auth.Attributes["path"]); p != "" {
		return expandPath(p)
	}
	return ""
}

func (e *KiroExecutor) metadataTokenPath(auth *cliproxyauth.Auth) string {
	if auth == nil || len(auth.Metadata) == 0 {
		return ""
	}
	if value, ok := auth.Metadata[kiroTokenPathMetadataKey]; ok {
		if path, ok := value.(string); ok && strings.TrimSpace(path) != "" {
			return path
		}
	}
	return ""
}

func (e *KiroExecutor) rotatorCandidates() []kiroTokenCandidate {
	if e == nil || e.tokenRotator == nil {
		return nil
	}
	return e.tokenRotator.candidates()
}

func (e *KiroExecutor) fallbackTokenCandidates(auth *cliproxyauth.Auth) []kiroTokenCandidate {
	if auth == nil {
		return nil
	}
	var candidates []kiroTokenCandidate
	names := []string{auth.FileName, auth.ID}
	base := ""
	if e.cfg != nil && strings.TrimSpace(e.cfg.AuthDir) != "" {
		base = expandPath(e.cfg.AuthDir)
	}
	seen := make(map[string]struct{})
	addCandidate := func(path string) {
		if path == "" {
			return
		}
		path = filepath.Clean(path)
		if _, exists := seen[path]; exists {
			return
		}
		if _, err := os.Stat(path); err == nil {
			seen[path] = struct{}{}
			candidates = append(candidates, kiroTokenCandidate{path: path})
		}
	}
	resolve := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if filepath.IsAbs(name) {
			addCandidate(name)
			return
		}
		if base != "" {
			addCandidate(filepath.Join(base, name))
		}
	}
	for _, name := range names {
		resolve(name)
	}
	for _, path := range discoverKiroTokenFiles(base) {
		addCandidate(path)
	}
	return candidates
}

func (e *KiroExecutor) applyCandidateSelection(auth *cliproxyauth.Auth, ts *authkiro.KiroTokenStorage, cand kiroTokenCandidate) {
	if auth == nil || ts == nil {
		return
	}
	auth.Runtime = ts
	auth.Metadata = attachTokenMetadata(auth.Metadata, ts)
	if cand.path != "" {
		if auth.Metadata == nil {
			auth.Metadata = make(map[string]any)
		}
		auth.Metadata[kiroTokenPathMetadataKey] = cand.path
	}
	if cand.region != "" {
		if auth.Attributes == nil {
			auth.Attributes = make(map[string]string)
		}
		auth.Attributes["region"] = cand.region
	}
}

func (e *KiroExecutor) advanceRotatorAfterAttempt(cand kiroTokenCandidate) {
	if e == nil || e.tokenRotator == nil || !cand.fromRotator {
		return
	}
	e.tokenRotator.advance(cand.rotatorIndex)
}

func (cand kiroTokenCandidate) describe() string {
	if strings.TrimSpace(cand.label) != "" {
		return fmt.Sprintf("token %s", cand.label)
	}
	if cand.path != "" {
		return fmt.Sprintf("token %s", cand.path)
	}
	return "token"
}

func discoverKiroTokenFiles(base string) []string {
	if strings.TrimSpace(base) == "" {
		return nil
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "kiro-") && strings.HasSuffix(lower, ".json") {
			paths = append(paths, filepath.Join(base, name))
		}
	}
	sort.Strings(paths)
	return paths
}

func (e *KiroExecutor) tokenFilePath(auth *cliproxyauth.Auth) string {
	if auth == nil {
		return ""
	}

	if path := e.metadataTokenPath(auth); path != "" {
		return path
	}

	if path := e.attributeTokenPath(auth); path != "" {
		return path
	}

	if e.cfg != nil && len(e.cfg.KiroTokenFiles) > 0 {
		tokenFile := e.cfg.KiroTokenFiles[0]
		if tokenFile.TokenFilePath != "" {
			return expandPath(tokenFile.TokenFilePath)
		}
	}

	// Fall back to default behavior
	candidates := []string{auth.FileName, auth.ID}
	base := ""
	if e.cfg != nil && strings.TrimSpace(e.cfg.AuthDir) != "" {
		base = expandPath(e.cfg.AuthDir)
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if filepath.IsAbs(candidate) {
			return candidate
		}
		if base != "" {
			path := filepath.Join(base, candidate)
			// Check if file exists before returning
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	if base != "" {
		if prefixed := discoverKiroTokenFiles(base); len(prefixed) > 0 {
			return prefixed[0]
		}
	}
	return ""
}

func expandPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}

func extractTokenFromMetadata(meta map[string]any) *authkiro.KiroTokenStorage {
	if len(meta) == 0 {
		return nil
	}
	ts := &authkiro.KiroTokenStorage{
		AccessToken:  kiroStringValue(meta["accessToken"], meta["access_token"]),
		RefreshToken: kiroStringValue(meta["refreshToken"], meta["refresh_token"]),
		ProfileArn:   kiroStringValue(meta["profileArn"], meta["profile_arn"]),
		AuthMethod:   kiroStringValue(meta["authMethod"], meta["auth_method"]),
		Provider:     kiroStringValue(meta["provider"]),
		Type:         "kiro",
	}
	if expires, ok := parseExpiry(meta["expiresAt"], meta["expires_at"]); ok {
		ts.ExpiresAt = expires
	}
	if ts.AccessToken == "" && ts.RefreshToken == "" {
		return nil
	}
	return ts
}

func attachTokenMetadata(meta map[string]any, ts *authkiro.KiroTokenStorage) map[string]any {
	if ts == nil {
		return meta
	}
	if meta == nil {
		meta = make(map[string]any)
	}
	meta["accessToken"] = ts.AccessToken
	meta["refreshToken"] = ts.RefreshToken
	meta["profileArn"] = ts.ProfileArn
	meta["authMethod"] = ts.AuthMethod
	meta["provider"] = ts.Provider
	if !ts.ExpiresAt.IsZero() {
		meta["expiresAt"] = ts.ExpiresAt.Format(time.RFC3339)
	}
	meta["type"] = "kiro"
	return meta
}

func kiroStringValue(values ...any) string {
	for _, value := range values {
		if str, ok := value.(string); ok && strings.TrimSpace(str) != "" {
			return str
		}
	}
	return ""
}

func parseExpiry(values ...any) (time.Time, bool) {
	for _, value := range values {
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				continue
			}
			if ts, err := time.Parse(time.RFC3339, v); err == nil {
				return ts, true
			}
			if unix, err := strconv.ParseInt(v, 10, 64); err == nil {
				return time.Unix(unix, 0), true
			}
		case float64:
			return time.Unix(int64(v), 0), true
		case int64:
			return time.Unix(v, 0), true
		case json.Number:
			if val, err := v.Int64(); err == nil {
				return time.Unix(val, 0), true
			}
		}
	}
	return time.Time{}, false
}

func estimateCompletionTokens(model, text string, toolCalls []kirotranslator.OpenAIToolCall) int64 {
	total := int64(0)

	// Assistant text
	if t, err := countTextTokens(model, text); err == nil {
		total += t
	} else {
		length := utf8.RuneCountInString(text)
		total += int64(math.Ceil(float64(length) / 4))
	}

	// Tool call arguments (JSON strings)
	for _, call := range toolCalls {
		if t, err := countTextTokens(model, call.Arguments); err == nil {
			total += t
		} else {
			length := utf8.RuneCountInString(call.Arguments)
			total += int64(math.Ceil(float64(length) / 4))
		}
	}

	if total < 1 {
		return 1
	}
	return total
}

func usageDetail(prompt, completion int64) usage.Detail {
	return usage.Detail{
		InputTokens:  prompt,
		OutputTokens: completion,
		TotalTokens:  prompt + completion,
	}
}

type kiroResult struct {
	Text             string
	ToolCalls        []kirotranslator.OpenAIToolCall
	KiroModel        string
	PromptTokens     int64
	CompletionTokens int64
	StreamChunks     [][]byte
}

type kiroStatusError struct {
	code int
	msg  string
}

func (e kiroStatusError) Error() string {
	if strings.TrimSpace(e.msg) != "" {
		return e.msg
	}
	return fmt.Sprintf("status %d", e.code)
}

func (e kiroStatusError) StatusCode() int { return e.code }
