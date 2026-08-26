#!/usr/bin/env bash
#MISE description="Download envtest binaries for tests"
set -euo pipefail

LOCALBIN=$(pwd)/.testbin

mkdir -p "${LOCALBIN}"

echo "Setting up envtest binaries for Kubernetes version ${ENVTEST_K8S_VERSION}..."
setup-envtest use "${ENVTEST_K8S_VERSION}" --bin-dir "${LOCALBIN}" || {
  echo "Error: Failed to set up envtest binaries for version ${ENVTEST_K8S_VERSION}."
  exit 1
}
