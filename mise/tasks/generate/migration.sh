#!/usr/bin/env bash
#MISE description="Run go generate"
#MISE depends_post=["fmt:go"]
set -euo pipefail

go generate ./...
