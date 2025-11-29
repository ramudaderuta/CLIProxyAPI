# Kiro Token Refresh Implementation Reference

## 1. Core Logic & Strategy

### Hybrid Refresh Model
*   **Buffer Constants:**
    *   `TokenExpirationBuffer` = **5 min** (Mandatory refresh window).
    *   `TokenEarlyRefreshBuffer` = **10 min** (Proactive best-effort window).
*   **Proactive Flow (`ValidateToken`):**
    1.  **< 5 min remaining:** Blocking refresh. Fail if refresh fails.
    2.  **< 10 min remaining:** Non-blocking refresh. Log warning on failure but return original token (Graceful Degradation).
    3.  **> 10 min remaining:** No action.

### Reactive Retry Flow (401 Handling)
Implemented in `Execute` / `ExecuteStream`:
1.  **Load Token:** Attempt non-blocking proactive refresh via `TryValidateToken`.
2.  **Request:** Send API request with current token.
3.  **401 Catch:** If HTTP 401 received:
    *   Trigger synchronous `RefreshToken`.
    *   Save new token.
    *   Retry request **once**.
    *   If retry fails, return error.

## 2. API Specifications & Critical Fixes

### Endpoint Conventions (Strict)
*   **Device Auth (`StartDeviceFlow`):** Uses **camelCase** (`clientId`, `clientSecret`).
*   **Token/Refresh (`RefreshToken`):** Uses **snake_case** (Standard OIDC).
    *   **Fix:** Changed `clientId` $\to$ `client_id`.
    *   **Fix:** Removed `clientSecret` (unless explicitly available in client file).
    *   **Fix:** Changed `grantType` $\to$ `grant_type`.

### Refresh Payload
```go
// Content-Type: application/x-www-form-urlencoded
payload := map[string]interface{}{
    "client_id":     f.clientID,
    "grant_type":    "refresh_token",
    "refresh_token": refreshToken,
    // client_secret included ONLY if present in client config
}
```

### Endpoint Selection
*   **BuilderId/IdC:** `https://oidc.{region}.amazonaws.com/token`
*   **Social/GitHub:** `https://prod.{region}.auth.desktop.kiro.dev/refreshToken`

## 3. Defensive Mechanisms

*   **UTC Normalization:** All expiry calcs use `time.Now().UTC()`.
*   **Latency Compensation:** Subtract **2 seconds** from `expires_in` to account for network RTT.
*   **Persistence:**
    *   If response contains `refresh_token`: Update storage (Rotation).
    *   If response empty: Retain existing `refresh_token`.
*   **Validation:** Reject `expires_in <= 0`.

## 4. Client Auto-Discovery & Optimization

*   **Lookup Strategy:**
    1.  **Targeted:** If `clientIdHash` exists in token, load `~/.cli-proxy-api/auth/<hash>.json` directly.
    2.  **Scan:** If no hash, scan directory for matching client config.
*   **Expiry Override:** When loading client files for refresh, **disable local expiry checks** (`checkExpiry=false`). Trust API response over local file metadata.
*   **Config:** `authDir` supports tilde expansion.

## 5. Debugging Cheat Sheet

*   **Invalid Grant:** usually wrong JSON casing or expired refresh token (>30 days).
*   **Invalid Client:** `client_id` in request $\neq$ `client_id` bound to refresh token.
*   **Verification:** Compare `expiresAt` in `kiro-auth-token.json` before/after refresh.