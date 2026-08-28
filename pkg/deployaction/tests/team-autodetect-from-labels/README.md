# Case: TEAM not set — must be auto-detected from metadata.labels.team

Pattern:
- Only `VAR: image=...`, `CLUSTER`, and `RESOURCE` are set — **no `TEAM`
  env var at all**. This is a common pattern where team detection is
  expected to work purely from the manifest.
- `metadata.labels.team: myteam` is a static (non-templated) scalar, so it
  can in principle be read directly from the raw, unrendered resource file
  before handlebars substitution happens.
- Contrasts with the `handlebars-loop-scalar-vars` testcase: in that case
  team auto-detection works fine too (namespace/labels are also static),
  but other scalar fields (`image`, `dashEnv`) are unquoted handlebars that
  break naive full-document YAML parsing. This case has NO other
  handlebars in metadata, only in `spec.image`, so it's a cleaner minimal
  reproduction of "detect team, ignore the rest of the templating".
- Also exercises `{{ image }}` combined with a realistic large `env` list,
  azure claims/groups, and accessPolicy.

Env vars used:
```
CLUSTER: prod-gcp
RESOURCE: prod-gcp.yaml
VAR: image=<built-image>
```
(No TEAM, no WORKLOAD_IMAGE.) Expected: team is detected as `myteam`.
