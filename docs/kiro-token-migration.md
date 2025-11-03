# Migrating Kiro Tokens to `kiro-token-file`

The Kiro provider now supports first-class configuration through the `kiro-token-file` block in `config.yaml`. This section explains how to move from legacy auto-detected `kiro-auth-token.json` files to the explicit configuration model.

## Why migrate?

- **Predictable precedence** – configured paths always win over auto-detected files in `auth-dir`.
- **Multi-account support** – declare multiple token files with per-entry labels and regions.
- **Improved observability** – configuration-sourced tokens are logged when registered.
- **Safe native imports** – legacy exports without `"type": "kiro"` are automatically enhanced in memory.

## Migration steps

1. **Locate your existing token files**
   - By default they reside in `~/.cli-proxy-api/kiro-auth-token.json`.
   - The file can live outside `auth-dir`; the new configuration accepts absolute or relative paths.
2. **Add a `kiro-token-file` block to `config.yaml`**
   ```yaml
   kiro-token-file:
     - token-file-path: "~/.cli-proxy-api/kiro-auth-token.json"
       region: "us-east-1"           # optional override if the token omits profileArn
       label: "primary-kiro-account" # optional friendly name used in logs/UIs
   ```
   - Multiple entries are supported; configure one block per account.
3. **(Optional) Remove auto-detected copies**
   - After the configuration is in place, copies left under `auth-dir` are ignored but can be deleted to avoid confusion.
4. **Restart or hot-reload the proxy**
   - `kiro-token-file` entries are picked up by the watcher and logged as `registered kiro token from configuration: ...`.

## Backward compatibility

- If both configured paths and auto-detected files exist, the configured paths take precedence.
- Kiro token files without `"type": "kiro"` no longer require manual edits—the runtime automatically enhances them while leaving the file unchanged on disk.
- Existing behaviour for `auth-dir` auto-detection remains available for quick local experiments.

## Troubleshooting

- Run `go test ./internal/config` (with a writable `GOCACHE`) to validate configuration parsing.
- Check the server logs for `registered kiro token` messages to confirm the active source (configuration vs auth directory).
- Validation errors raised during startup will include the index of the failing `kiro-token-file` entry.
