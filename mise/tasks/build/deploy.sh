#!/usr/bin/env bash
#MISE description="Build deploy CLI"
set -euo pipefail

LAST_COMMIT=$(git rev-parse --short HEAD)
DATE=$(date "+%Y-%m-%d")
BUILDTIME=$(date "+%s")
LDFLAGS="-X github.com/nais/deploy/pkg/version.Revision=${LAST_COMMIT} -X github.com/nais/deploy/pkg/version.Date=${DATE} -X github.com/nais/deploy/pkg/version.BuildUnixTime=${BUILDTIME}"
go build -o bin/deploy -ldflags "-s ${LDFLAGS}" cmd/deploy/main.go
