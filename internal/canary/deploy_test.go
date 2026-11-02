package canary

import (
	"strings"
	"testing"
	"time"
)

func TestRenderManifest(t *testing.T) {
	now := time.Unix(1700000000, 0)
	m, err := renderManifest(now, "ghcr.io/nais/deploy/canary:test")
	if err != nil {
		t.Fatalf("renderManifest: %v", err)
	}

	if got := m["kind"]; got != "Application" {
		t.Errorf("kind = %v, want Application", got)
	}

	meta, ok := m["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata not a map: %T", m["metadata"])
	}
	if got := meta["name"]; got != "nais-deploy-canary" {
		t.Errorf("name = %v, want nais-deploy-canary", got)
	}
	if got := meta["namespace"]; got != TeamSlug {
		t.Errorf("namespace = %v, want %s", got, TeamSlug)
	}

	spec := m["spec"].(map[string]any)
	env := spec["env"].([]any)
	first := env[0].(map[string]any)
	if first["name"] != "DEPLOY_START" {
		t.Errorf("first env var = %v, want DEPLOY_START", first["name"])
	}
	wantStart := "1700000000000000000"
	if first["value"] != wantStart {
		t.Errorf("DEPLOY_START = %v, want %s", first["value"], wantStart)
	}

	if got := spec["image"]; got != "ghcr.io/nais/deploy/canary:test" {
		t.Errorf("image = %v, want ghcr.io/nais/deploy/canary:test", got)
	}
}

func TestRenderAlerts(t *testing.T) {
	m, err := renderAlerts()
	if err != nil {
		t.Fatalf("renderAlerts: %v", err)
	}

	if got := m["kind"]; got != "PrometheusRule" {
		t.Errorf("kind = %v, want PrometheusRule", got)
	}

	meta := m["metadata"].(map[string]any)
	if got := meta["namespace"]; got != "nais-verification" {
		t.Errorf("namespace = %v, want nais-verification", got)
	}

	spec := m["spec"].(map[string]any)
	groups := spec["groups"].([]any)
	rules := groups[0].(map[string]any)["rules"].([]any)
	first := rules[0].(map[string]any)
	if !strings.Contains(first["expr"].(string), "nais_deploy_canary_time_since_deploy") {
		t.Errorf("expr missing metric: %v", first["expr"])
	}
}
