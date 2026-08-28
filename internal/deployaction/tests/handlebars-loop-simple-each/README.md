# Case: Handlebars {{#each}} loops (ingresses + env) with a VARS file

Pattern:
- Resource file uses `{{image}}` scalar substitution AND two `{{#each}}`
  block loops:
  - `{{#each ingresses as |url|}} - {{url}} {{/each}}` — loop over a plain
    string array.
  - `{{#each env}} - name: {{this.name}} value: {{this.value}} {{/each}}` —
    loop over a list of objects, referencing fields via `this.*`.
- The VARS file (`vars-dev.yaml`) supplies `ingresses` but deliberately
  omits `env` — this exercises the "loop over a key that isn't defined in
  vars" edge case (should render as an empty list, not error).
- Image is supplied via `IMAGE` (injected as the `image` template
  variable), not `WORKLOAD_IMAGE`.

Env vars used:
```
CLUSTER: dev-gcp
RESOURCE: nais.yaml
VARS: vars-dev.yaml
IMAGE: <built-image>
```
(No TEAM.) Expected: team is auto-detected as `myteam` from
`metadata.labels.team`.

Also a regression test: the unquoted `image: {{image}}` scalar breaks a
naive YAML parse of the raw file, so TEAM auto-detection must happen on the
rendered/templated output, not the raw source file (see
`handlebars-loop-scalar-vars` for more detail on this bug/fix).
