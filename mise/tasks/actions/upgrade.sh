#!/usr/bin/env bash
#MISE description="Upgrade all github actions to latest"
set -euo pipefail

go tool github.com/sethvargo/ratchet upgrade .github/workflows/*.yaml
