package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "os"
    kiro "github.com/router-for-me/CLIProxyAPI/v6/internal/translator/kiro"
    authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
)

func main() {
    flag.Parse()
    if flag.NArg() < 1 {
        fmt.Fprintln(os.Stderr, "usage: show_kiro_request <jsonfile>")
        os.Exit(2)
    }
    path := flag.Arg(0)
    b, err := os.ReadFile(path)
    if err != nil { panic(err) }
    tok := &authkiro.KiroTokenStorage{AccessToken: "token", AuthMethod: "social", ProfileArn: "arn:aws:codewhisperer:us-east-1:699475941385:profile/EHGA3GRVQMUK"}
    out, err := kiro.BuildRequest("claude-sonnet-4-5", b, tok, map[string]any{})
    if err != nil { panic(err) }
    var pretty map[string]any
    if err := json.Unmarshal(out, &pretty); err != nil { panic(err) }
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    _ = enc.Encode(pretty)
}

