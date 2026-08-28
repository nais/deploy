#!/usr/bin/env bash
set -euo pipefail

# === Nais Deploy v3 ===
# Drop-in replacement for nais/deploy/actions/deploy@v2.
#
# All templating, team auto-detection, and deployment logic lives in the
# deploy-action Go binary (github.com/nais/deploy/cmd/deploy-action). This
# script only downloads that binary and executes it, passing through the
# same environment variables as v2: CLUSTER, RESOURCE, IMAGE, WORKLOAD_IMAGE,
# VARS, VAR, TEAM, WAIT, TIMEOUT, DRY_RUN.

VERSION="$(cat "$(dirname "${BASH_SOURCE[0]}")/version")"
BINARY="/tmp/deploy-action"

if [ ! -f "$BINARY" ]; then
  echo "::group::Download deploy-action" >&2
  url="https://github.com/nais/deploy/releases/download/${VERSION}/deploy-action-linux"
  echo "Downloading deploy-action from ${url}..." >&2
  curl -sSL -f -o "$BINARY" "$url"
  chmod +x "$BINARY"
  echo "::endgroup::" >&2
fi

exec "$BINARY"
