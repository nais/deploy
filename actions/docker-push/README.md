# Usage

```yaml
- uses: nais/deploy/actions/docker-push@v2
  id: docker-push
  with:
    config: ${{ vars.DOCKER_PUSH_CONFIG }}
```

## Configuration

The intention is for the organization admin to create a single secret containing required information.

```json
{
  "gar_service_account": "<service account email>",
  "gar_registry_url": "<registry url without scheme>",
  "workload_identity_provider": "<workload identity provider>"
}
```
