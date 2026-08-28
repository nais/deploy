#!/usr/bin/env bash
set -euo pipefail

# --- Helper to trigger a single testcase against the deploy-action Go binary ---
#
# Each testcase directory contains:
#   env.txt    - KEY=VALUE lines defining RESOURCE, CLUSTER, WORKLOAD_IMAGE/VAR/VARS, etc.
#   *.yaml     - resource file(s) referenced (relatively) by RESOURCE in env.txt
#
# Usage:
#   ./run.sh <case-name> [--deploy]
#   ./run.sh --list
#
# By default runs with DRY_RUN=true (just renders resources, no cluster interaction).
# Pass --deploy to actually run deploy-action without forcing DRY_RUN (requires
# nais CLI to be configured/authenticated).
#
# This builds cmd/deploy-action from source and runs it directly, rather than
# going through actions/deploy/entrypoint.sh, since that script downloads a
# released binary from GitHub and isn't useful for local iteration.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${SCRIPT_DIR}/.."
BINARY="${REPO_ROOT}/bin/deploy-action"

usage() {
  echo "Usage: $0 <case-name> [--deploy]"
  echo "       $0 --list"
  echo ""
  echo "Available cases:"
  list_cases
}

list_cases() {
  for d in "${SCRIPT_DIR}"/*/; do
    name=$(basename "$d")
    [ -f "${d}env.txt" ] && echo "  - ${name}"
  done
}

if [ "${1:-}" = "--list" ] || [ -z "${1:-}" ]; then
  usage
  exit 0
fi

CASE_NAME="$1"
shift || true
CASE_DIR="${SCRIPT_DIR}/${CASE_NAME}"
DRY_RUN=true

for arg in "$@"; do
  case "$arg" in
    --deploy) DRY_RUN=false ;;
    *) echo "Unknown option: $arg" >&2; exit 1 ;;
  esac
done

if [ ! -d "$CASE_DIR" ]; then
  echo "::error::Unknown case '${CASE_NAME}'." >&2
  usage
  exit 1
fi

if [ ! -f "${CASE_DIR}/env.txt" ]; then
  echo "::error::Missing env.txt in ${CASE_DIR}" >&2
  exit 1
fi

echo "Building deploy-action..." >&2
( cd "$REPO_ROOT" && go build -buildvcs=false -o bin/deploy-action ./cmd/deploy-action )

echo "=== Running testcase: ${CASE_NAME} (DRY_RUN=${DRY_RUN}) ==="

# Load env.txt into a clean subshell environment and run deploy-action with the
# case directory as CWD, so relative RESOURCE paths resolve correctly.
(
  cd "$CASE_DIR"
  set -a
  # shellcheck disable=SC1091
  source ./env.txt
  set +a
  export DRY_RUN
  exec "$BINARY"
)

