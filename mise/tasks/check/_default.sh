#!/usr/bin/env bash
#MISE description="Run all static analysis tools"
unset MISE_TASK_OUTPUT && mise run check:staticcheck ::: check:vulncheck ::: check:deadcode ::: check:gosec
