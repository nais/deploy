package canary

import (
	"testing"
	"time"
)

func TestRenderManifest(t *testing.T) {
	now := time.Unix(1700000000, 0)
	m, err := renderManifest(now)
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
}
