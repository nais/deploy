// Package deployaction implements the "Nais Deploy v3" pipeline used by
// actions/deploy: handlebars-template one or more Kubernetes resource files,
// auto-detect the owning team, split multi-document YAML into individual
// files, and apply each of them with `nais alpha apply`.
//
// It replaces the logic that used to live in actions/deploy/entrypoint.sh,
// reusing the templating and annotation-injection building blocks already
// implemented locally instead of shelling out to a separately downloaded CLI
// and parsing its output with yq/jq.
package deployaction

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/nais/deploy/internal/githubactions"
)

// Config holds the same inputs as the v2/v3 GitHub Action environment
// variables: CLUSTER, RESOURCE, TEAM, VARS, VAR, IMAGE, WORKLOAD_IMAGE,
// WAIT, TIMEOUT, DRY_RUN.
type Config struct {
	Cluster       string
	Resource      []string
	Team          string
	VariablesFile string
	Variables     []string
	Image         string
	WorkloadImage string
	Wait          bool
	Timeout       time.Duration
	DryRun        bool

	// OutputDir is where rendered, per-resource YAML files are written. If
	// empty, a temporary directory is created and removed after Run returns.
	OutputDir string

	// NaisPath is the path to the `nais` CLI binary used for `nais alpha
	// apply`. Defaults to "nais" (resolved via PATH).
	NaisPath string
}

// ConfigFromEnv builds a Config from the same environment variables used by
// actions/deploy/entrypoint.sh.
func ConfigFromEnv() (*Config, error) {
	cfg := &Config{
		Cluster:       os.Getenv("CLUSTER"),
		Team:          os.Getenv("TEAM"),
		VariablesFile: os.Getenv("VARS"),
		Image:         os.Getenv("IMAGE"),
		WorkloadImage: os.Getenv("WORKLOAD_IMAGE"),
		Wait:          getEnvBool("WAIT", true),
		DryRun:        getEnvBool("DRY_RUN", false),
		NaisPath:      "nais",
	}

	if resource := os.Getenv("RESOURCE"); len(resource) > 0 {
		for r := range strings.SplitSeq(resource, ",") {
			cfg.Resource = append(cfg.Resource, strings.TrimSpace(r))
		}
	}

	if v := os.Getenv("VAR"); len(v) > 0 {
		for kv := range strings.SplitSeq(v, ",") {
			cfg.Variables = append(cfg.Variables, strings.TrimSpace(kv))
		}
	}

	timeout := os.Getenv("TIMEOUT")
	if len(timeout) == 0 {
		timeout = "10m"
	}
	d, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid TIMEOUT %q: %w", timeout, err)
	}
	cfg.Timeout = d

	return cfg, nil
}

func getEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if len(v) == 0 {
		return def
	}
	return strings.EqualFold(v, "true")
}

func (cfg *Config) Validate() error {
	if len(cfg.Resource) == 0 {
		return fmt.Errorf("RESOURCE is required")
	}
	for _, r := range cfg.Resource {
		if len(r) == 0 {
			return fmt.Errorf("empty entry in RESOURCE (check for stray commas)")
		}
	}
	if len(cfg.Cluster) == 0 {
		return fmt.Errorf("CLUSTER is required")
	}
	return nil
}

// EffectiveImage returns the image to apply via `--set spec.image=`.
// IMAGE (used as a template variable) takes priority over WORKLOAD_IMAGE.
func (cfg *Config) EffectiveImage() string {
	if len(cfg.Image) > 0 {
		return cfg.Image
	}
	return cfg.WorkloadImage
}

// RenderedResource is a single Kubernetes resource, already templated and
// annotated, and written to disk as plain YAML ready to be applied.
type RenderedResource struct {
	Kind string
	Path string
}

// Run executes the full templating + team-detection + apply pipeline.
func Run(cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		githubactions.Error("%s", err)
		return err
	}

	end := githubactions.Group("Nais Deploy v3")
	log.Infof("Cluster: %s", cfg.Cluster)
	log.Infof("Resources: %s", strings.Join(cfg.Resource, ","))
	if len(cfg.Team) > 0 {
		log.Infof("Team: %s", cfg.Team)
	}
	image := cfg.EffectiveImage()
	if len(image) > 0 {
		log.Infof("Image: %s", image)
	} else {
		log.Infof("Image: (will use currently running image)")
	}
	log.Infof("Wait: %t", cfg.Wait)
	log.Infof("Timeout: %s", cfg.Timeout)
	end()

	outputDir := cfg.OutputDir
	if len(outputDir) == 0 {
		dir, err := os.MkdirTemp("", "deploy-rendered-*")
		if err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
		defer os.RemoveAll(dir)
		outputDir = dir
	}

	rendered, err := Render(cfg, outputDir)
	if err != nil {
		githubactions.Error("%s", err)
		return err
	}

	if cfg.DryRun {
		endDry := githubactions.Group("Dry run - rendered resources")
		for _, r := range rendered {
			content, err := os.ReadFile(r.Path) // #nosec G304 -- path constructed from our own temp dir
			if err != nil {
				endDry()
				return err
			}
			fmt.Printf("=== %s ===\n%s\n", filepath.Base(r.Path), content)
		}
		endDry()
		log.Info("Dry run complete. No deployment performed.")
		return nil
	}

	endDeploy := githubactions.Group("Deploy with nais CLI")
	defer endDeploy()
	for _, r := range rendered {
		if err := apply(cfg, r); err != nil {
			githubactions.Error("%s", err)
			return err
		}
	}

	return nil
}

// Render templates all of cfg.Resource, writes the resulting per-resource
// YAML files into outputDir, and (if cfg.Team is unset) auto-detects and
// sets cfg.Team from the rendered output. It performs no cluster
// interaction, which makes it usable both by Run (for dry-run/apply) and
// directly in tests to verify templating/team-detection behavior.
func Render(cfg *Config, outputDir string) ([]RenderedResource, error) {
	templateVariables, err := loadTemplateVariables(cfg)
	if err != nil {
		return nil, err
	}

	endTemplate := githubactions.Group("Template resources")
	rendered := make([]RenderedResource, 0)
	for i, path := range cfg.Resource {
		resources, err := multiDocumentFileAsJSON(path, templateVariables)
		if err != nil {
			endTemplate()
			return nil, fmt.Errorf("failed to template resource %q: %w", path, err)
		}

		files, err := writeResources(resources, outputDir, i)
		if err != nil {
			endTemplate()
			return nil, err
		}
		if len(files) == 0 {
			endTemplate()
			return nil, fmt.Errorf("no applicable resources after filtering for: %s", path)
		}
		rendered = append(rendered, files...)
	}
	endTemplate()

	// Auto-detect TEAM from the rendered (guaranteed handlebars-free) files,
	// never from raw source: a resource file can contain handlebars syntax
	// anywhere (e.g. `image: {{ image }}`), which would confuse a plain YAML
	// parser even when the team/namespace fields themselves are static.
	if len(cfg.Team) == 0 {
		cfg.Team = detectTeam(rendered)
		if len(cfg.Team) == 0 {
			return nil, fmt.Errorf("TEAM is required. Set it via env var or metadata.labels.team in your resource file")
		}
		log.Infof("Detected team: %s", cfg.Team)
	}

	return rendered, nil
}

func loadTemplateVariables(cfg *Config) (templateVariables, error) {
	vars := templateVariables{}
	if len(cfg.VariablesFile) > 0 {
		var err error
		vars, err = templateVariablesFromFile(cfg.VariablesFile)
		if err != nil {
			return nil, fmt.Errorf("load template variables: %w", err)
		}
	}

	if len(cfg.Variables) > 0 {
		overrides := templateVariablesFromSlice(cfg.Variables)
		maps.Copy(vars, overrides)
	}

	// IMAGE is injected as the `image` template variable, mirroring the
	// behavior of the old bash entrypoint's prepare_vars_file step.
	if len(cfg.Image) > 0 {
		vars["image"] = cfg.Image
	}

	return vars, nil
}

// writeResources converts each templated JSON resource to YAML and writes it
// to outputDir, skipping any generated `kind: Image` resource (handled
// instead via `nais alpha apply --set spec.image=`).
func writeResources(resources []json.RawMessage, outputDir string, fileIndex int) ([]RenderedResource, error) {
	out := make([]RenderedResource, 0, len(resources))

	for i, resource := range resources {
		var meta struct {
			Kind       string `json:"kind"`
			APIVersion string `json:"apiVersion"`
		}
		if err := json.Unmarshal(resource, &meta); err != nil {
			return nil, fmt.Errorf("parse resource kind: %w", err)
		}

		if meta.Kind == "Image" && meta.APIVersion == "nais.io/v1" {
			log.Infof("Skipping generated Image resource (handled via --set spec.image)")
			continue
		}

		content, err := yaml.JSONToYAML(resource)
		if err != nil {
			return nil, fmt.Errorf("convert resource to yaml: %w", err)
		}

		path := filepath.Join(outputDir, fmt.Sprintf("%d-%d.yaml", fileIndex, i))
		if err := os.WriteFile(path, content, 0o600); err != nil {
			return nil, fmt.Errorf("write rendered resource: %w", err)
		}

		log.Infof("  -> %s (%s)", path, meta.Kind)
		out = append(out, RenderedResource{Kind: meta.Kind, Path: path})
	}

	return out, nil
}

func detectTeam(resources []RenderedResource) string {
	for _, r := range resources {
		content, err := os.ReadFile(r.Path) // #nosec G304 -- path constructed from our own temp dir
		if err != nil {
			continue
		}
		asJSON, err := yaml.YAMLToJSON(content)
		if err != nil {
			continue
		}
		if team := detectTeamFromResource(asJSON); len(team) > 0 {
			log.Infof("Detected team %q from %s", team, r.Path)
			return team
		}
		if team := detectNamespace(asJSON); len(team) > 0 {
			log.Infof("Detected team %q from namespace in %s", team, r.Path)
			return team
		}
	}
	return ""
}

// apply runs `nais alpha apply` for a single rendered resource file.
func apply(cfg *Config, r RenderedResource) error {
	args := []string{
		"apply", r.Path,
		"--environment", cfg.Cluster,
		"--allow-ignored-fields",
	}

	if len(cfg.Team) > 0 {
		args = append(args, "--team", cfg.Team)
	}

	if cfg.Wait {
		args = append(args, "--wait", "--timeout", cfg.Timeout.String())
	}

	// Only set spec.image on workload resources (Application, Naisjob), not
	// on ConfigMaps or other supporting resources.
	image := cfg.EffectiveImage()
	if len(image) > 0 && (r.Kind == "Application" || r.Kind == "Naisjob") {
		args = append(args, "--set", "spec.image="+image)
	}

	log.Infof("Running: %s %s", cfg.NaisPath, strings.Join(args, " "))

	cmd := exec.Command(cfg.NaisPath, args...) // #nosec G204 -- args constructed internally, not from unsanitized user input
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("deploy %s: %w", filepath.Base(r.Path), err)
	}

	return nil
}
