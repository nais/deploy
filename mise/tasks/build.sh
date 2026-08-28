#!/usr/bin/env bash
#MISE description="Build deploy-action"
set -euo pipefail

go build -o bin/deploy-action .
