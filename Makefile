PROTOC = $(shell which protoc)
PROTOC_GEN_GO = $(shell which protoc-gen-go)

.PHONY: all proto hookd deployd token-generator deploy provision alpine test docker upload

all: hookd deployd deploy provision

proto:
	wget -O deployment.proto https://raw.githubusercontent.com/navikt/protos/master/deployment/deployment.proto
	$(PROTOC) --plugin=$(PROTOC_GEN_GO) --go_out=plugins=grpc:. deployment.proto
	mv deployment.pb.go pkg/pb/
	rm -f deployment.proto

fmt:
	go fmt github.com/navikt/deployment/...

hookd:
	go build -o bin/hookd cmd/hookd/main.go

deployd:
	go build -o bin/deployd cmd/deployd/main.go

deploy:
	go build -o bin/deploy cmd/deploy/main.go

deploy-release-linux:
	GOOS=linux \
	GOARCH=amd64 \
	go build -o deploy-linux -ldflags="-s -w" cmd/deploy/main.go

deploy-release-darwin:
	GOOS=darwin \
	GOARCH=amd64 \
	go build -o deploy-darwin -ldflags="-s -w" cmd/deploy/main.go

deploy-release-windows:
	GOOS=windows \
	GOARCH=amd64 \
	go build -o deploy-windows -ldflags="-s -w" cmd/deploy/main.go

provision:
	go build -o bin/provision cmd/provision/*.go

alpine:
	go build -a -installsuffix cgo -o bin/hookd cmd/hookd/main.go
	go build -a -installsuffix cgo -o bin/deployd cmd/deployd/main.go
	go build -a -installsuffix cgo -o bin/deploy cmd/deploy/main.go
	go build -a -installsuffix cgo -o bin/provision cmd/provision/*.go

test:
	go test ./... -count=1

docker:
	docker build -t navikt/deployment:latest .

upload:
	docker push navikt/deployment:latest

migration:
	go generate ./...
