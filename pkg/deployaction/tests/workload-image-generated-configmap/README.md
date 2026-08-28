# Case: WORKLOAD_IMAGE with multiple RESOURCE files, one generated at CI time

Pattern:
- `RESOURCE` is a comma-separated list of TWO files: a generated ConfigMap
  resource file (produced by e.g. a `yq` step embedding another YAML's
  contents as a string field) and the Application manifest.
- Exercises: comma-separated multi-file RESOURCE handling, a ConfigMap-only
  file (kind != Application/Naisjob, so spec.image must NOT be set on it),
  alongside a regular Application file.

Env vars used:
```
CLUSTER: prod-gcp
RESOURCE: configmap.yaml,app.yaml
WORKLOAD_IMAGE: <built-image>
```
