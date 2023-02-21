//go:build tools
// +build tools

package tools

import (
	_ "github.com/vektra/mockery/v2"
	_ "golang.org/x/vuln/cmd/govulncheck"
	_ "honnef.co/go/tools/cmd/staticcheck"
	_ "mvdan.cc/gofumpt"
  _ "google.golang.org/protobuf/cmd/protoc-gen-go"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
)
