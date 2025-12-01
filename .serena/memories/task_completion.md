# Task Completion Checklist
- Run the smallest relevant tests; for translator/Kiro changes: `go test ./tests/unit/kiro -run 'BuildRequest|ParseResponse' -count=1` (and stream mappers as needed). For broader edits consider `go test ./tests/unit/... ./tests/regression/... -race -cover`.
- If touching auth header plumbing/iFlow, run `go test ./tests/unit/iflow -run TestIFlowExecutorForwardsCustomHeaders -count=1`.
- For streaming/translation updates, optionally replay fixtures via `go test ./tests/unit/kiro -run 'ConvertKiroStreamToAnthropic|NormalizeKiroStreamPayload' -count=1`.
- Keep code formatted (`gofmt`), lint (`go vet`, `golangci-lint` if available), and ensure configs/fixtures updated.
- Summarize changes and note any skipped tests with rationale.