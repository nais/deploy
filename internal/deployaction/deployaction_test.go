package deployaction_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/nais/deploy/internal/deployaction"
)

// These tests exercise deployaction.Render against the fixtures under
// tests/ in this package, mirroring what actions/deploy/entrypoint.sh (now
// cmd/deploy-action) does when invoked with the corresponding env.txt
// environment variables. Each fixture models a distinct combination of
// WORKLOAD_IMAGE/IMAGE/VAR/VARS, handlebars templating, and team
// auto-detection, generalized from patterns commonly seen in real
// deployment manifests.

const testcasesDir = "./tests"

// resourcePaths resolves resource file names to paths relative to the given
// testcase directory, as the RESOURCE env var (comma-separated) would.
func resourcePaths(dir string, names ...string) []string {
	paths := make([]string, len(names))
	for i, n := range names {
		paths[i] = filepath.Join(testcasesDir, dir, n)
	}
	return paths
}

// kindsOf returns the Kind field of each rendered resource, in order.
func kindsOf(t *testing.T, rendered []deployaction.RenderedResource) []string {
	t.Helper()
	kinds := make([]string, len(rendered))
	for i, r := range rendered {
		kinds[i] = r.Kind
	}
	return kinds
}

// decodeResource reads and unmarshals a rendered resource file's YAML into a
// generic map for field assertions.
func decodeResource(t *testing.T, path string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(content, &doc))
	return doc
}

func nestedString(t *testing.T, doc map[string]any, path ...string) string {
	t.Helper()
	var cur any = doc
	for _, key := range path {
		m, ok := cur.(map[string]any)
		require.Truef(t, ok, "expected map while traversing %v at %q, got %T", path, key, cur)
		cur, ok = m[key]
		require.Truef(t, ok, "key %q not found while traversing %v", key, path)
	}
	s, ok := cur.(string)
	require.Truef(t, ok, "expected string at %v, got %T", path, cur)
	return s
}

func TestRender_HandlebarsVarSingleApp(t *testing.T) {
	// Single Application, image supplied via VAR (as the `image` template
	// variable), TEAM set explicitly.
	dir := "handlebars-var-single-app"
	cfg := &deployaction.Config{
		Cluster:   "dev-gcp",
		Resource:  resourcePaths(dir, "nais.yaml"),
		Team:      "yolo",
		Variables: []string{"image=europe-north1-docker.pkg.dev/example-project/myteam/myapp:2024.01.01-abcdef1"},
	}

	rendered, err := deployaction.Render(cfg, t.TempDir())
	require.NoError(t, err)
	require.Len(t, rendered, 1)
	assert.Equal(t, []string{"Application"}, kindsOf(t, rendered))
	assert.Equal(t, "yolo", cfg.Team, "explicit TEAM must not be overridden")

	doc := decodeResource(t, rendered[0].Path)
	assert.Equal(t, "europe-north1-docker.pkg.dev/example-project/myteam/myapp:2024.01.01-abcdef1",
		nestedString(t, doc, "spec", "image"))
}

func TestRender_WorkloadImageMultidoc(t *testing.T) {
	// Multi-document YAML (Postgres + Application) in a single file,
	// WORKLOAD_IMAGE (not a template var -- applied later via
	// --set spec.image=), TEAM auto-detected from the Application's
	// metadata.labels.team since Postgres has no team label.
	dir := "workload-image-multidoc"
	cfg := &deployaction.Config{
		Cluster:       "prod-gcp",
		Resource:      resourcePaths(dir, "nais.yaml"),
		WorkloadImage: "europe-north1-docker.pkg.dev/example-project/myteam/myapp:2024.01.01-abcdef1",
	}

	rendered, err := deployaction.Render(cfg, t.TempDir())
	require.NoError(t, err)
	require.Len(t, rendered, 2, "expected both Postgres and Application resources")
	assert.ElementsMatch(t, []string{"Postgres", "Application"}, kindsOf(t, rendered))
	assert.Equal(t, "myteam", cfg.Team)

	// The Application resource's image must NOT be templated in (there's no
	// {{ image }} in this manifest); it's applied later via --set spec.image=.
	for _, r := range rendered {
		if r.Kind != "Application" {
			continue
		}
		doc := decodeResource(t, r.Path)
		_, hasImage := doc["spec"].(map[string]any)["image"]
		assert.False(t, hasImage, "Application resource should not have spec.image set by templating")
	}
}

func TestRender_WorkloadImagePlainApp(t *testing.T) {
	// Plain Application with no handlebars syntax at all, and explicit
	// TEAM. Verifies the "no-op templating" path.
	dir := "workload-image-plain-app"
	cfg := &deployaction.Config{
		Cluster:       "dev-gcp",
		Resource:      resourcePaths(dir, "naiserator-dev.yaml"),
		Team:          "yolo",
		WorkloadImage: "europe-north1-docker.pkg.dev/example-project/myteam/myapp:2024.01.01-abcdef1",
	}

	rendered, err := deployaction.Render(cfg, t.TempDir())
	require.NoError(t, err)
	require.Len(t, rendered, 1)
	assert.Equal(t, "Application", rendered[0].Kind)
	assert.Equal(t, "yolo", cfg.Team)
}

func TestRender_WorkloadImageGeneratedConfigmap(t *testing.T) {
	// Two separate RESOURCE files -- a CI-generated ConfigMap and the
	// Application referencing it via filesFrom.
	dir := "workload-image-generated-configmap"
	cfg := &deployaction.Config{
		Cluster:       "prod-gcp",
		Resource:      resourcePaths(dir, "configmap.yaml", "app.yaml"),
		WorkloadImage: "europe-north1-docker.pkg.dev/example-project/myteam/myapp:2024.01.01-abcdef1",
	}

	rendered, err := deployaction.Render(cfg, t.TempDir())
	require.NoError(t, err)
	require.Len(t, rendered, 2)
	assert.ElementsMatch(t, []string{"ConfigMap", "Application"}, kindsOf(t, rendered))
	assert.Equal(t, "myteam", cfg.Team)
}

func TestRender_HandlebarsLoopSimpleEach(t *testing.T) {
	// {{#each ingresses}} and {{#each env}} (with `this.*` field
	// references) driven by a VARS file. The VARS file intentionally
	// omits the `env` key to exercise the empty-loop edge case (no `env`
	// entries should be rendered, not an error).
	dir := "handlebars-loop-simple-each"
	cfg := &deployaction.Config{
		Cluster:       "dev-gcp",
		Resource:      resourcePaths(dir, "nais.yaml"),
		VariablesFile: filepath.Join(testcasesDir, dir, "vars-dev.yaml"),
		Image:         "europe-north1-docker.pkg.dev/example-project/myteam/myapp:2024.01.01-abcdef1",
	}

	rendered, err := deployaction.Render(cfg, t.TempDir())
	require.NoError(t, err)
	require.Len(t, rendered, 1)
	assert.Equal(t, "myteam", cfg.Team, "team must be auto-detected from metadata.labels.team")

	doc := decodeResource(t, rendered[0].Path)
	spec := doc["spec"].(map[string]any)
	assert.Equal(t, "europe-north1-docker.pkg.dev/example-project/myteam/myapp:2024.01.01-abcdef1", spec["image"])

	ingresses, ok := spec["ingresses"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"https://myapp.ansatt.example.com"}, ingresses)

	// env is absent from vars-dev.yaml -> the each-loop must render as empty,
	// not error out or leave literal handlebars syntax behind.
	env, ok := spec["env"]
	if ok {
		assert.Empty(t, env)
	}
}

func TestRender_HandlebarsLoopScalarVars(t *testing.T) {
	// Scalar vars embedded directly inside strings (e.g. `{{ dashEnv }}`),
	// plus loops using implicit field access (no `this.` prefix), driven
	// by a large realistic VARS payload.
	dir := "handlebars-loop-scalar-vars"
	cfg := &deployaction.Config{
		Cluster:       "dev-fss",
		Resource:      resourcePaths(dir, "nais.yaml"),
		VariablesFile: filepath.Join(testcasesDir, dir, "vars-q.yaml"),
		Variables:     []string{"image=europe-north1-docker.pkg.dev/example-project/myteam/myapp:2024.01.01-abcdef1"},
	}

	rendered, err := deployaction.Render(cfg, t.TempDir())
	require.NoError(t, err)
	require.Len(t, rendered, 1)
	assert.NotEmpty(t, cfg.Team, "team must be auto-detected")

	doc := decodeResource(t, rendered[0].Path)
	spec := doc["spec"].(map[string]any)
	assert.Equal(t, "europe-north1-docker.pkg.dev/example-project/myteam/myapp:2024.01.01-abcdef1", spec["image"])
}

func TestRender_TeamAutodetectFromLabels(t *testing.T) {
	// TEAM is never supplied as an env var and must be discovered from
	// metadata.labels.team.
	dir := "team-autodetect-from-labels"
	cfg := &deployaction.Config{
		Cluster:   "prod-gcp",
		Resource:  resourcePaths(dir, "prod-gcp.yaml"),
		Variables: []string{"image=europe-north1-docker.pkg.dev/example-project/myteam/myapp:2024.01.01-abcdef1"},
	}

	rendered, err := deployaction.Render(cfg, t.TempDir())
	require.NoError(t, err)
	require.Len(t, rendered, 1)
	assert.Equal(t, "myteam", cfg.Team)
}

func TestRender_MultiVar(t *testing.T) {
	// Multiple VAR entries in a single comma-separated env var
	// (VAR=min_replicas=2,max_replicas=3), each substituted into distinct
	// scalar template fields.
	dir := "multi-var"
	cfg := &deployaction.Config{
		Cluster:       "dev-gcp",
		Resource:      resourcePaths(dir, "naiserator-dev.yaml"),
		Team:          "yolo",
		WorkloadImage: "europe-north1-docker.pkg.dev/example-project/myteam/myapp:2024.01.01-abcdef1",
		Variables:     []string{"min_replicas=2", "max_replicas=3"},
	}

	rendered, err := deployaction.Render(cfg, t.TempDir())
	require.NoError(t, err)
	require.Len(t, rendered, 1)

	doc := decodeResource(t, rendered[0].Path)
	replicas := doc["spec"].(map[string]any)["replicas"].(map[string]any)
	assert.EqualValues(t, 2, replicas["min"])
	assert.EqualValues(t, 3, replicas["max"])
}

// TestRender_TeamRequired verifies that when TEAM cannot be auto-detected
// (no metadata.labels.team and no metadata.namespace anywhere) and none was
// supplied explicitly, Render returns an error instead of silently
// deploying without a team.
func TestRender_TeamRequired(t *testing.T) {
	dir := t.TempDir()
	resourceFile := filepath.Join(dir, "nais.yaml")
	require.NoError(t, os.WriteFile(resourceFile, []byte("apiVersion: nais.io/v1alpha1\nkind: Application\nmetadata:\n  name: myapp\nspec:\n  port: 8080\n"), 0o600))

	cfg := &deployaction.Config{
		Cluster:  "dev-gcp",
		Resource: []string{resourceFile},
	}

	_, err := deployaction.Render(cfg, t.TempDir())
	assert.Error(t, err)
}
