# Case: Handlebars templating via VAR (single Application)

Pattern:
- Single `Application` resource.
- Image is injected via handlebars placeholder `{{ image }}` in the resource file.
- The GitHub Action passes `VAR: image=<built-image>` (not `WORKLOAD_IMAGE`),
  so the template variable must be substituted rather than using `--set spec.image`.

Env vars used:
```
CLUSTER: dev-gcp
RESOURCE: nais.yaml
VAR: image=<built-image>
```
