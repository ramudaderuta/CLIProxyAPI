package executor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

const (
	kiroBaseURLTemplate    = "https://codewhisperer.%s.amazonaws.com/generateAssistantResponse"
	kiroAmazonQURLTemplate = "https://codewhisperer.%s.amazonaws.com/SendMessageStreaming"
	kiroDefaultRegion      = "us-east-1"
	kiroAgentPrefix        = "aws-sdk-js/1.0.7"
	kiroIDEVersion         = "KiroIDE-0.1.25"
)

type kiroClient struct {
	cfg     *config.Config
	auth    *authkiro.KiroAuth
	macOnce sync.Once
	macHash string
}

func newKiroClient(cfg *config.Config) *kiroClient {
	return &kiroClient{
		cfg:  cfg,
		auth: authkiro.NewKiroAuth(),
	}
}

func (c *kiroClient) ensureToken(ctx context.Context, token *authkiro.KiroTokenStorage) error {
	if token == nil {
		return fmt.Errorf("kiro client: token storage missing")
	}
	proxyURL := ""
	if c.cfg != nil {
		proxyURL = c.cfg.ProxyURL
	}
	if _, err := c.auth.GetAuthenticatedClient(ctx, token, proxyURL); err != nil {
		return fmt.Errorf("kiro client: auth refresh failed: %w", err)
	}
	return nil
}

func (c *kiroClient) doRequest(ctx context.Context, auth *cliproxyauth.Auth, token *authkiro.KiroTokenStorage, regionOverride string, model string, body []byte) ([]byte, int, http.Header, error) {
	if err := c.ensureToken(ctx, token); err != nil {
		return nil, 0, nil, err
	}

	c.debugDumpPayload("kiro request", body)

	endpoint := c.buildEndpoint(model, token.ProfileArn, regionOverride)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, nil, err
	}
	c.applyHeaders(req, token.AccessToken)

	if auth != nil {
		recordAPIRequest(ctx, c.cfg, upstreamRequestLog{
			URL:       endpoint,
			Method:    http.MethodPost,
			Headers:   req.Header.Clone(),
			Body:      body,
			Provider:  "kiro",
			AuthID:    auth.ID,
			AuthLabel: auth.Label,
		})
	}

	httpClient := newProxyAwareHTTPClient(ctx, c.cfg, auth, 120*time.Second)
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		recordAPIResponseError(ctx, c.cfg, err)
		return nil, 0, nil, err
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Errorf("kiro client: close body error: %v", errClose)
		}
	}()

	recordAPIResponseMetadata(ctx, c.cfg, resp.StatusCode, resp.Header.Clone())
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		recordAPIResponseError(ctx, c.cfg, err)
		return nil, resp.StatusCode, resp.Header.Clone(), err
	}
	data = kirotranslator.NormalizeKiroStreamPayload(data)
	c.debugDumpPayload("kiro response", data)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		appendAPIResponseChunk(ctx, c.cfg, data)
		return nil, resp.StatusCode, resp.Header.Clone(), kiroStatusError{code: resp.StatusCode, msg: string(data)}
	}

	appendAPIResponseChunk(ctx, c.cfg, data)
	return data, resp.StatusCode, resp.Header.Clone(), nil
}

func (c *kiroClient) buildEndpoint(model, profileArn, regionOverride string) string {
	region := c.extractRegion(regionOverride, profileArn)
	if strings.HasPrefix(strings.ToLower(model), "amazonq-") {
		return fmt.Sprintf(kiroAmazonQURLTemplate, region)
	}
	return fmt.Sprintf(kiroBaseURLTemplate, region)
}

func (c *kiroClient) extractRegion(regionOverride, profileArn string) string {
	if trimmed := strings.TrimSpace(regionOverride); trimmed != "" {
		return trimmed
	}
	parts := strings.Split(profileArn, ":")
	if len(parts) > 3 {
		region := parts[3]
		if strings.HasPrefix(region, "us") || strings.HasPrefix(region, "eu") || strings.HasPrefix(region, "ap") {
			return region
		}
	}
	return kiroDefaultRegion
}

func (c *kiroClient) applyHeaders(req *http.Request, token string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	agentSuffix := c.macHashValue()
	req.Header.Set("x-amz-user-agent", fmt.Sprintf("%s %s-%s", kiroAgentPrefix, kiroIDEVersion, agentSuffix))
	req.Header.Set("user-agent", fmt.Sprintf("%s ua/2.1 os/cli lang/go api/codewhispererstreaming#1.0.7 m/E %s-%s", kiroAgentPrefix, kiroIDEVersion, agentSuffix))
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
}

func (c *kiroClient) macHashValue() string {
	c.macOnce.Do(func() {
		interfaces, err := net.Interfaces()
		if err != nil {
			c.macHash = "0000000000000000"
			return
		}
		for _, iface := range interfaces {
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addr := iface.HardwareAddr.String()
			if addr == "" {
				continue
			}
			sum := sha256.Sum256([]byte(addr))
			c.macHash = hex.EncodeToString(sum[:])
			return
		}
		c.macHash = "0000000000000000"
	})
	return c.macHash
}

func (c *kiroClient) debugDumpPayload(label string, payload []byte) {
	if c.cfg == nil || !c.cfg.Debug || len(payload) == 0 {
		return
	}
	const limit = 4096
	dump := bytes.TrimSpace(payload)
	truncated := false
	if len(dump) > limit {
		dump = append([]byte{}, dump[:limit]...)
		truncated = true
	} else {
		dump = append([]byte{}, dump...)
	}
	render := sanitizePayloadForLog(dump)
	if render == "" {
		render = "[binary payload omitted]"
	}
	log.WithFields(log.Fields{
		"provider":  "kiro",
		"bytes":     len(payload),
		"truncated": truncated,
	}).Debugf("%s payload: %s", label, render)
}

func sanitizePayloadForLog(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}

	out := make([]byte, 0, len(payload))
	lastWasCR := false

	for _, b := range payload {
		switch {
		case b == '\r':
			if !lastWasCR {
				out = append(out, '\n')
			}
			lastWasCR = true
			continue
		case b == '\n':
			if lastWasCR {
				lastWasCR = false
				continue
			}
			out = append(out, '\n')
			continue
		}

		lastWasCR = false
		switch {
		case b == '\t':
			out = append(out, b)
		case b < 0x20:
			continue
		case b == 0x7f:
			continue
		case b >= 0x80 && b < 0xa0:
			continue
		default:
			out = append(out, b)
		}
	}

	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return ""
	}
	return string(out)
}
