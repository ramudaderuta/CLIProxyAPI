package main

import (
	"encoding/json"
	"fmt"
	"log"

	kirotranslator "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	"time"
)

func main() {
	payload := `{
		"messages": [
			{"role": "user", "content": "Use calculator"},
			{"role": "assistant", "content": [
				{"type": "tool_use", "id": "calc_1", "name": "calculator", "input": {"expression": "2+2"}}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "calc_1", "content": "4"}
			]}
		]
	}`

	var payloadMap map[string]any
	if err := json.Unmarshal([]byte(payload), &payloadMap); err != nil {
		log.Fatal(err)
	}

	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		log.Fatal(err)
	}

	token := &authkiro.KiroTokenStorage{
		ProfileArn:  "arn:aws:codewhisperer:us-east-1:699475941385:profile/test",
		AccessToken: "test_access_token",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Type:        "kiro",
	}

	result, err := kirotranslator.BuildRequest("claude-sonnet-4-5", payloadBytes, token, nil)
	if err != nil {
		log.Fatal(err)
	}

	var request map[string]any
	if err := json.Unmarshal(result, &request); err != nil {
		log.Fatal(err)
	}

	prettyJSON, _ := json.MarshalIndent(request, "", "  ")
	fmt.Println(string(prettyJSON))
}