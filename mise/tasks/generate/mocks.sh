#!/usr/bin/env bash
#MISE description="Generate mocks"
#MISE depends_post=["fmt:go"]
set -euo pipefail

go tool github.com/vektra/mockery/v2 --inpackage --all --case snake --srcpkg ./pkg/hookd/database
go tool github.com/vektra/mockery/v2 --inpackage --all --case snake --srcpkg ./pkg/grpc/dispatchserver
go tool github.com/vektra/mockery/v2 --inpackage --all --case snake --srcpkg ./pkg/pb
