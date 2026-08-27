#!/usr/bin/env bash
#MISE description="Generate protobuf code"
#MISE depends_post=["fmt:go"]
set -euo pipefail

protoc --go-grpc_opt=paths=source_relative --go_opt=paths=source_relative --go_out=. --go-grpc_out=. pkg/pb/deployment.proto
