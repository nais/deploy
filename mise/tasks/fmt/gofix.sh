#!/usr/bin/env bash
#MISE description="Run go fix on all packages"
set -euo pipefail

go fix ./...
