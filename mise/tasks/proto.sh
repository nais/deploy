#!/usr/bin/env bash
#MISE description="Generate protobuf code"
protoc --go-grpc_opt=paths=source_relative --go_opt=paths=source_relative --go_out=. --go-grpc_out=. pkg/pb/deployment.proto
