# Nais Deploy

This repository contains the `nais/deploy` GitHub Action. The action renders
Nais resource manifests with Handlebars template variables and deploys them
with the [Nais CLI](https://doc.nais.io/cli).

For platform-user documentation, see [Build and deploy](https://doc.nais.io/build/how-to/build-and-deploy).

## Use the action

Deploy a resource file from a GitHub Actions workflow:

```yaml
steps:
  - uses: actions/checkout@v5

  - uses: nais/deploy/actions/deploy@v3-alpha
    env:
      CLUSTER: dev-gcp
      RESOURCE: nais.yaml
```

`CLUSTER` and `RESOURCE` are required. `RESOURCE` accepts a comma-separated
list of YAML or JSON files. Each file can contain multiple YAML documents.

The action detects `TEAM` from `metadata.labels.team` or `metadata.namespace`
when it is not set explicitly. It waits for completion by default.

### Configuration

| Environment variable | Default | Description                                                                                             |
| --- | --- |---------------------------------------------------------------------------------------------------------|
| `CLUSTER` | Required | Nais cluster to deploy to.                                                                              |
| `RESOURCE` | Required | Comma-separated YAML or JSON resource files.                                                            |
| `TEAM` | Auto-detected | Deploying team. Detected from a resource's `metadata.labels.team` or `metadata.namespace` when omitted. |
| `VARS` | Unset | YAML or JSON file containing Handlebars template variables.                                             |
| `VAR` | Unset | Comma-separated `key=value` template variables. Values override matching variables from `VARS`.         |
| `IMAGE` | Unset | Value supplied as the `image` template variable and used as the workload image.                         |
| `WORKLOAD_IMAGE` | Unset | Workload image when `IMAGE` is not set.                                                                 |
| `WAIT` | `true` | Wait for the deployment to finish.                                                                      |
| `TIMEOUT` | `10m` | Maximum wait time when `WAIT` is enabled.                                                               |
| `DRY_RUN` | `false` | Render and validate resources without deploying them.                                                   |

`IMAGE` takes precedence over `WORKLOAD_IMAGE`. When an image is set, it is
applied to `Application` and `Naisjob` resources only.

The complete action reference is also available in
[`actions/deploy/README.md`](actions/deploy/README.md).

## Development

The repository uses [mise](https://mise.jdx.dev/) to install the pinned Go

```bash
mise run build
mise run test
mise run check
mise run fmt
```

`mise run build` produces `bin/deploy-action`. The test suite is self-contained
and can also be run with `go test ./...` when the required Go version is
available locally.

GitHub Actions runs the build, tests, and individual static-analysis checks on
pull requests. Pushes to `deploy-v3` also release the action version declared
in [`actions/deploy/version`](actions/deploy/version).

## Repository layout

| Path | Purpose |
| --- | --- |
| `actions/deploy/` | Composite GitHub Action and its user-facing documentation. |
| `internal/deployaction/` | Rendering, team detection, and `nais alpha apply` implementation. |
| `internal/deployaction/tests/` | Manifest fixtures used by the Go test suite. |
| `mise/tasks/` | Build, formatting, test, and static-analysis tasks. |
