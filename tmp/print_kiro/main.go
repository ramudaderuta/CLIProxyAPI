//go:build printkiro_cli

package main

import (
	"encoding/json"
	"fmt"
	auth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	kiro "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: print_kiro <fixture>")
		os.Exit(2)
	}
	payload, err := os.ReadFile(os.Args[1])
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
