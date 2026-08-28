# Case: WORKLOAD_IMAGE, plain single-document Application (no templating)

Pattern:
- Single-document `Application` manifest, no `{{ }}` handlebars anywhere.
- No IMAGE/VAR/VARS set — only `WORKLOAD_IMAGE`, so templating is a no-op
  pass-through (no variables to substitute), with image applied via
  `nais alpha apply --set spec.image=`.
- Exercises the "no actual templating happens, but still routed through the
  same pipeline for consistent TEAM auto-detection" path, plus richer
  real-world spec fields (scalingStrategy.kafka, accessPolicy with external
  host, azure, kafka pool, observability autoInstrumentation).

Env vars used:
```
CLUSTER: dev-gcp
RESOURCE: naiserator-dev.yaml
WORKLOAD_IMAGE: <built-image>
```

Team is auto-detected from `metadata.labels.team` (`myteam`).
