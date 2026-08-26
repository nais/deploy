#!/usr/bin/env bash
#MISE description="Build hookd"
LAST_COMMIT=$(git rev-parse --short HEAD)
DATE=$(date "+%Y-%m-%d")
BUILDTIME=$(date "+%s")
LDFLAGS="-X github.com/nais/deploy/pkg/version.Revision=${LAST_COMMIT} -X github.com/nais/deploy/pkg/version.Date=${DATE} -X github.com/nais/deploy/pkg/version.BuildUnixTime=${BUILDTIME}"
go build -o bin/hookd -ldflags "-s ${LDFLAGS}" cmd/hookd/main.go
