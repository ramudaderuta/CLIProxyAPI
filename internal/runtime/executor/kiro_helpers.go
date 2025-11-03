package executor

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func (e *KiroExecutor) tokenStorageFromAuth(auth *cliproxyauth.Auth) (*authkiro.KiroTokenStorage, error) {
	if auth == nil {
		return nil, fmt.Errorf("kiro executor: auth is nil")
	}
	if ts, ok := auth.Runtime.(*authkiro.KiroTokenStorage); ok && ts != nil {
		return ts, nil
	}
	if token := extractTokenFromMetadata(auth.Metadata); token != nil {
		auth.Runtime = token
		return token, nil
	}
	path := e.tokenFilePath(auth)
	if path == "" {
		return nil, fmt.Errorf("kiro executor: token path unavailable for %s", auth.ID)
	}
	ts, err := authkiro.LoadTokenFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("kiro executor: load token: %w", err)
	}
	auth.Runtime = ts
	return ts, nil
}

func (e *KiroExecutor) tokenFilePath(auth *cliproxyauth.Auth) string {
	if auth == nil {
		return ""
	}

	// First check for explicitly configured token files
	if e.cfg != nil && len(e.cfg.KiroTokenFiles) > 0 {
		// Use the first configured token file (could be enhanced later to match by region/label)
		tokenFile := e.cfg.KiroTokenFiles[0]
		if tokenFile.TokenFilePath != "" {
			return expandPath(tokenFile.TokenFilePath)
		}
	}

	// Fall back to auth attributes
	if auth.Attributes != nil {
		if p := strings.TrimSpace(auth.Attributes["path"]); p != "" {
			return expandPath(p)
		}
	}

	// Fall back to default behavior
	candidates := []string{auth.FileName, auth.ID, "kiro-auth-token.json"}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if filepath.IsAbs(candidate) {
			return candidate
		}
		if e.cfg != nil && e.cfg.AuthDir != "" {
			path := filepath.Join(expandPath(e.cfg.AuthDir), candidate)
			// Check if file exists before returning
			if _, err := os.Stat(path); err == nil {
				return path
			}
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

func estimatePromptTokens(model string, payload []byte) (int64, error) {
	enc, err := tokenizerForModel(model)
	if err != nil {
		return 0, err
	}
	return countOpenAIChatTokens(enc, payload)
}

func estimateCompletionTokens(text string, toolCalls []kirotranslator.OpenAIToolCall) int64 {
	length := utf8.RuneCountInString(text)
	for _, call := range toolCalls {
		length += utf8.RuneCountInString(call.Arguments)
	}
	tokens := math.Ceil(float64(length) / 4)
	if tokens < 1 {
		return 1
	}
	return int64(tokens)
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
