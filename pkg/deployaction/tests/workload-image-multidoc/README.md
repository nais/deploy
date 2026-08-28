# Case: WORKLOAD_IMAGE with multi-document YAML (Postgres + Application)

Pattern:
- A single resource file contains TWO YAML documents separated by `---`:
  a `Postgres` resource and an `Application` resource.
- No handlebars placeholders at all — image is applied purely via
  `WORKLOAD_IMAGE` / `--set spec.image=` on the Application document only.
- Exercises: multi-document splitting (one file per
  `kubernetes.resources[]` entry, regardless of whether templating is
  used), and only setting spec.image on the workload kind
  (Application/Naisjob), not on Postgres.

Env vars used:
```
CLUSTER: prod-gcp
RESOURCE: nais.yaml
WORKLOAD_IMAGE: <built-image>
```

Team is auto-detected from `metadata.labels.team` on the Application document
(`myteam`), since TEAM is not explicitly set.
