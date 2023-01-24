PROTOC = $(shell which protoc)
PROTOC_GEN_GO = $(shell which protoc-gen-go)
BUILDTIME = $(shell date "+%s")
DATE = $(shell date "+%Y-%m-%d")
LAST_COMMIT = $(shell git rev-parse --short HEAD)
VERSION ?= $(DATE)-$(LAST_COMMIT)
LDFLAGS := -X github.com/nais/deploy/pkg/version.Revision=$(LAST_COMMIT) -X github.com/nais/deploy/pkg/version.Date=$(DATE) -X github.com/nais/deploy/pkg/version.BuildUnixTime=$(BUILDTIME)
arch := amd64
os := $(shell uname -s | tr '[:upper:]' '[:lower:]')

.PHONY: all proto hookd deployd token-generator deploy provision alpine test docker upload

all: hookd deployd deploy provision

install-protobuf-go:
	go install google.golang.org/protobuf/cmd/protoc-gen-go
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc

proto:
	$(PROTOC) --go-grpc_opt=paths=source_relative --go_opt=paths=source_relative --go_out=. --go-grpc_out=. pkg/pb/deployment.proto

hookd:
	go build -o bin/hookd -ldflags "-s $(LDFLAGS)" cmd/hookd/main.go

deployd:
	go build -o bin/deployd -ldflags "-s $(LDFLAGS)" cmd/deployd/main.go

deploy:
	go build -o bin/deploy -ldflags "-s $(LDFLAGS)" cmd/deploy/main.go

crypt:
	go build -o bin/crypt -ldflags "-s $(LDFLAGS)" cmd/crypt/main.go

provision:
	go build -o bin/provision -ldflags "-s $(LDFLAGS)" cmd/provision/main.go

mocks:
	cd pkg/hookd/database && mockery --inpackage --all --case snake
	cd pkg/grpc/deployserver && mockery --inpackage --all --case snake
	cd pkg/grpc/dispatchserver && mockery --inpackage --all --case snake
	cd pkg/pb && mockery --inpackage --all --case snake

deploy-release-linux:
	GOOS=linux \
	GOARCH=amd64 \
	go build -o deploy-linux -ldflags="-s -w $(LDFLAGS)" cmd/deploy/main.go

deploy-release-darwin:
	GOOS=darwin \
	GOARCH=amd64 \
	go build -o deploy-darwin -ldflags="-s -w $(LDFLAGS)" cmd/deploy/main.go

deploy-release-windows:
	GOOS=windows \
	GOARCH=amd64 \
	go build -o deploy-windows -ldflags="-s -w $(LDFLAGS)" cmd/deploy/main.go

alpine:
	go build -a -installsuffix cgo -o bin/hookd -ldflags "-s $(LDFLAGS)" cmd/hookd/main.go
	go build -a -installsuffix cgo -o bin/deployd -ldflags "-s $(LDFLAGS)" cmd/deployd/main.go
	go build -a -installsuffix cgo -o bin/deploy -ldflags "-s $(LDFLAGS)" cmd/deploy/main.go
	go build -a -installsuffix cgo -o bin/provision -ldflags "-s $(LDFLAGS)" cmd/provision/*.go

test:
	go test ./... -count=1

migration:
	go generate ./...

kubebuilder:
	curl -L https://github.com/kubernetes-sigs/kubebuilder/releases/download/v2.3.1/kubebuilder_2.3.1_${os}_${arch}.tar.gz | tar -xz -C /tmp/
	mv /tmp/kubebuilder_2.3.1_${os}_${arch} /usr/local/kubebuilder
