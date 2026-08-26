#!/usr/bin/env bash
#MISE description="Run all tests"
#MISE depends=["setup-envtest"]
set -euo pipefail

go test ./...
