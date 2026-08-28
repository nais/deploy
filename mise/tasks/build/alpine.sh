#!/usr/bin/env bash
#MISE description="Build all binaries for Alpine (CGO disabled)"
set -euo pipefail

LAST_COMMIT=$(git rev-parse --short HEAD)
DATE=$(date "+%Y-%m-%d")
BUILDTIME=$(date "+%s")
LDFLAGS="-X github.com/nais/deploy/pkg/version.Revision=${LAST_COMMIT} -X github.com/nais/deploy/pkg/version.Date=${DATE} -X github.com/nais/deploy/pkg/version.BuildUnixTime=${BUILDTIME}"
go build -a -installsuffix cgo -o bin/hookd -ldflags "-s ${LDFLAGS}" ./cmd/hookd/
go build -a -installsuffix cgo -o bin/deployd -ldflags "-s ${LDFLAGS}" ./cmd/deployd/
go build -a -installsuffix cgo -o bin/deploy -ldflags "-s ${LDFLAGS}" ./cmd/deploy/
go build -a -installsuffix cgo -o bin/deploy-action -ldflags "-s ${LDFLAGS}" ./cmd/deploy-action/
