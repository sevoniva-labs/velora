// casdoor-automation-check performs a read-only, sanitized check of the
// optional Casdoor application automation API. It never creates, updates or
// disables an application.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/casdooradmin"
)

func main() {
	baseURL := strings.TrimSpace(os.Getenv("VELORA_CASDOOR_ADMIN_URL"))
	token := strings.TrimSpace(os.Getenv("VELORA_CASDOOR_AUTOMATION_TOKEN"))
	ref := strings.TrimSpace(os.Getenv("VELORA_CASDOOR_AUTOMATION_REF"))
	if baseURL == "" || token == "" || ref == "" {
		fail("VELORA_CASDOOR_ADMIN_URL, VELORA_CASDOOR_AUTOMATION_TOKEN and VELORA_CASDOOR_AUTOMATION_REF are required")
	}
	client, err := casdooradmin.New(casdooradmin.Config{BaseURL: baseURL, Token: token, Enabled: true})
	if err != nil {
		fail(err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	app, found, err := client.GetApplication(ctx, ref)
	if err != nil {
		fail(err.Error())
	}
	output := map[string]any{"status": "ok", "found": found}
	if found {
		// Deliberately emit only non-secret application metadata.
		output["application"] = map[string]any{"name": app.Name, "organization": app.Organization, "client_id_present": strings.TrimSpace(app.ClientID) != "", "redirect_uri_count": len(app.RedirectURIs), "grant_type_count": len(app.GrantTypes), "enabled": app.Enabled}
	}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	_, _ = fmt.Fprintln(os.Stderr, "casdoor-automation-check failed:", message)
	os.Exit(1)
}
