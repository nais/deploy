PROTOC = $(shell which protoc)
PROTOC_GEN_GO = $(shell which protoc-gen-go)
BUILDTIME = $(shell date "+%s")
DATE = $(shell date "+%Y-%m-%d")
K8S_VERSION := 1.27.1
LAST_COMMIT = $(shell git rev-parse --short HEAD)
VERSION ?= $(DATE)-$(LAST_COMMIT)
LDFLAGS := -X github.com/nais/deploy/pkg/version.Revision=$(LAST_COMMIT) -X github.com/nais/deploy/pkg/version.Date=$(DATE) -X github.com/nais/deploy/pkg/version.BuildUnixTime=$(BUILDTIME)
NAIS_API_COMMIT_SHA := e1c532d516dfdd586dc98e6f7e5275d91c53dcf5
NAIS_API_TARGET_DIR=pkg/naisapi/protoapi
arch := $(shell uname -m | sed s/aarch64/arm64/ | sed s/x86_64/amd64/)
os := $(shell uname -s | tr '[:upper:]' '[:lower:]')
testbin_dir := ./.testbin/
tools_archive := kubebuilder-tools-${K8S_VERSION}-$(os)-$(arch).tar.gz

.PHONY: all proto hookd deployd token-generator deploy alpine test docker upload deploy-alpine hookd-alpine deployd-alpine

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

test: kubebuilder
	go test ./... -count=1

migration:
	go generate ./...

kubebuilder: $(testbin_dir)/$(tools_archive)
	tar -xzf $(testbin_dir)/$(tools_archive) --strip-components=2 -C $(testbin_dir)
	chmod -R +x $(testbin_dir)

$(testbin_dir)/$(tools_archive):
	mkdir -p $(testbin_dir)
	curl -L -O --output-dir $(testbin_dir) "https://storage.googleapis.com/kubebuilder-tools/$(tools_archive)"

check:
	go run honnef.co/go/tools/cmd/staticcheck ./...

deployd-alpine:
	go build -a -installsuffix cgo -o bin/deployd -ldflags "-s $(LDFLAGS)" ./cmd/deployd/

hookd-alpine:
	go build -a -installsuffix cgo -o bin/hookd -ldflags "-s $(LDFLAGS)" ./cmd/hookd/

deploy-alpine:
	go build -a -installsuffix cgo -o bin/deploy -ldflags "-s $(LDFLAGS)" ./cmd/deploy/

generate-nais-api:
	mkdir -p ./$(NAIS_API_TARGET_DIR)
	wget -O ./$(NAIS_API_TARGET_DIR)/teams.proto https://raw.githubusercontent.com/nais/api/$(NAIS_API_COMMIT_SHA)/pkg/protoapi/schema/teams.proto
	wget -O ./$(NAIS_API_TARGET_DIR)/users.proto https://raw.githubusercontent.com/nais/api/$(NAIS_API_COMMIT_SHA)/pkg/protoapi/schema/users.proto
	wget -O ./$(NAIS_API_TARGET_DIR)/pagination.proto https://raw.githubusercontent.com/nais/api/$(NAIS_API_COMMIT_SHA)/pkg/protoapi/schema/pagination.proto
	$(PROTOC) \
		--proto_path=$(NAIS_API_TARGET_DIR) \
		--go_opt=Mpagination.proto=github.com/nais/deploy/$(NAIS_API_TARGET_DIR) \
		--go_opt=Musers.proto=github.com/nais/deploy/$(NAIS_API_TARGET_DIR) \
		--go_opt=Mteams.proto=github.com/nais/deploy/$(NAIS_API_TARGET_DIR) \
		--go_opt=paths=source_relative \
		--go_out=$(NAIS_API_TARGET_DIR) \
		--go-grpc_opt=Mpagination.proto=github.com/nais/deploy/$(NAIS_API_TARGET_DIR) \
		--go-grpc_opt=Musers.proto=github.com/nais/deploy/$(NAIS_API_TARGET_DIR) \
		--go-grpc_opt=Mteams.proto=github.com/nais/deploy/$(NAIS_API_TARGET_DIR) \
		--go-grpc_opt=paths=source_relative \
		--go-grpc_out=$(NAIS_API_TARGET_DIR) \
		$(NAIS_API_TARGET_DIR)/*.proto
	rm -f $(NAIS_API_TARGET_DIR)/*.proto
