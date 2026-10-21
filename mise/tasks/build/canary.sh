#!/usr/bin/env bash
#MISE description="Build the canary binary"
set -euo pipefail

go build -o bin/canary ./cmd/canary
