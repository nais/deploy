# Case: Handlebars loops + scalar vars (implicit field refs, no `this.`)

Pattern:
- Resource file mixes scalar handlebars substitutions used in multiple
  places, including inside strings/paths: `{{ dashEnv }}` (appears in
  `metadata.name`, `envFrom` secret name, and a vault kvPath string), and
  `{{ vaultKvEnv }}`.
- `{{#each ingresses as |url|}} - {{url}} {{/each}}` — loop over a plain
  string array (block param form).
- `{{#each envvars}} - name: {{ name }} value: "{{ value }}" {{/each}}` —
  loop over a list of objects using IMPLICIT field access (no `this.`
  prefix, unlike the loop-simple-each case), plus a static env entry
  appended after the loop closes.
- VARS file supplies `vaultKvEnv`, `ingresses` (4 entries) and `envvars`
  (9 objects) — a much larger/more realistic vars payload than the other
  loop testcase. `dashEnv` is deliberately left undefined to exercise
  handlebars' "render as empty string" behavior for missing scalars.
- Image supplied via `VAR: image=...`.

Env vars used:
```
CLUSTER: dev-fss
RESOURCE: nais.yaml
VARS: vars-q.yaml
VAR: image=<built-image>
```
(No TEAM.) Expected: team is auto-detected as `myteam` from
`metadata.labels.team`.

Note: this case is a regression test for a real bug. The resource file has
an *unquoted* `{{ image }}`/`{{ dashEnv }}` mustache in scalar positions
(e.g. `image: {{ image }}`), which makes a plain YAML parser mis-parse the
RAW, untemplated file (e.g. as an invalid flow-mapping). Detecting TEAM by
parsing the raw file directly — as the old bash entrypoint used to do —
fails here even though `metadata.labels.team` itself is a perfectly static
value. The fix is to always render the file through the templating engine
first, then run TEAM detection against the rendered (handlebars-free)
output.
