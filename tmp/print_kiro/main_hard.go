//go:build printkiro_hard

package main

import (
	"encoding/json"
	auth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	kiro "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	"os"
)

func main() {
	payload, err := os.ReadFile("tests/shared/testdata/nonstream/test_hard_request.json")
	if err != nil {
		panic(err)
	}
	token := &auth.KiroTokenStorage{AccessToken: "test"}
	body, err := kiro.BuildRequest("claude-sonnet-4-5", payload, token, nil)
	if err != nil {
		panic(err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		panic(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(out)
}
