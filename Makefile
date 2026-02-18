PROTOC = $(shell which protoc)
PROTOC_GEN_GO = $(shell which protoc-gen-go)
BUILDTIME = $(shell date "+%s")
DATE = $(shell date "+%Y-%m-%d")
K8S_VERSION := 1.33.5
LAST_COMMIT = $(shell git rev-parse --short HEAD)
VERSION ?= $(DATE)-$(LAST_COMMIT)
LDFLAGS := -X github.com/nais/deploy/pkg/version.Revision=$(LAST_COMMIT) -X github.com/nais/deploy/pkg/version.Date=$(DATE) -X github.com/nais/deploy/pkg/version.BuildUnixTime=$(BUILDTIME)
arch := $(shell uname -m | sed s/aarch64/arm64/ | sed s/x86_64/amd64/)
os := $(shell uname -s | tr '[:upper:]' '[:lower:]')
testbin_dir := ./.testbin/
tools_archive := kubebuilder-tools-${K8S_VERSION}-$(os)-$(arch).tar.gz
SETUP_ENVTEST := $(shell command -v setup-envtest 2>/dev/null || command -v $(shell go env GOPATH)/bin/setup-envtest 2>/dev/null)

.PHONY: all proto hookd deployd token-generator deploy alpine test docker upload deploy-alpine hookd-alpine deployd-alpine envtest-info

all: hookd deployd deploy

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

mocks:
	go run github.com/vektra/mockery/v2 --inpackage --all --case snake --srcpkg ./pkg/hookd/database
	go run github.com/vektra/mockery/v2 --inpackage --all --case snake --srcpkg ./pkg/grpc/dispatchserver
	go run github.com/vektra/mockery/v2 --inpackage --all --case snake --srcpkg ./pkg/pb

fmt:
	go run mvdan.cc/gofumpt -w ./


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

test:
	@if [ -n "$(SETUP_ENVTEST)" ]; then \
		ASSETS=$$($(SETUP_ENVTEST) use -p path); \
		echo "Using envtest assets: $$ASSETS"; \
		export KUBEBUILDER_ASSETS=$$ASSETS; \
		go test ./... -count=1; \
	else \
		echo "setup-envtest not found; falling back to kubebuilder tools download"; \
		$(MAKE) kubebuilder; \
		export KUBEBUILDER_ASSETS=$(testbin_dir); \
		go test ./... -count=1; \
	fi

migration:
	go generate ./...

kubebuilder: $(testbin_dir)/$(tools_archive)
	tar -xzf $(testbin_dir)/$(tools_archive) --strip-components=2 -C $(testbin_dir)
	chmod -R +x $(testbin_dir)

$(testbin_dir)/$(tools_archive):
	mkdir -p $(testbin_dir)
	curl -fL --output $(testbin_dir)/$(tools_archive) "https://storage.googleapis.com/kubebuilder-tools/$(tools_archive)"

check:
	go run honnef.co/go/tools/cmd/staticcheck ./...

deployd-alpine:
	go build -a -installsuffix cgo -o bin/deployd -ldflags "-s $(LDFLAGS)" ./cmd/deployd/

hookd-alpine:
	go build -a -installsuffix cgo -o bin/hookd -ldflags "-s $(LDFLAGS)" ./cmd/hookd/

deploy-alpine:
	go build -a -installsuffix cgo -o bin/deploy -ldflags "-s $(LDFLAGS)" ./cmd/deploy/

envtest-info:
	@echo "setup-envtest (PATH): $$(command -v setup-envtest || true)"
	@echo "setup-envtest (GOPATH/bin): $$(command -v $(shell go env GOPATH)/bin/setup-envtest || true)"
	@echo "GOPATH: $$(go env GOPATH)"
	@echo "KUBEBUILDER_ASSETS (env): $${KUBEBUILDER_ASSETS:-<unset>}"
	@if [ -n "$(SETUP_ENVTEST)" ]; then \
		ASSETS=$$($(SETUP_ENVTEST) use -p path); \
		echo "envtest assets: $$ASSETS"; \
		ls -la "$$ASSETS"; \
	else \
		echo "setup-envtest not found"; \
	fi
