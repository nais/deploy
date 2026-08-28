# Case: Multiple VAR entries in a single comma-separated env var

Pattern:
- `VAR=min_replicas=2,max_replicas=3` sets two distinct scalar template
  variables in one comma-separated env var, each substituted into a
  different field (`spec.replicas.min` / `spec.replicas.max`).
- Also exercises inconsistent whitespace around handlebars braces
  (`{{ min_replicas }}` vs `{{ max_replicas}}`) still resolving correctly.

Env vars used:
```
CLUSTER: dev-gcp
RESOURCE: naiserator-dev.yaml
WORKLOAD_IMAGE: <built-image>
TEAM: yolo
VAR: min_replicas=2,max_replicas=3
```
