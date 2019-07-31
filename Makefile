PROTOC = $(shell which protoc)
PROTOC_GEN_GO = $(shell which protoc-gen-go)
HOOKD_ALPINE_LDFLAGS := -X github.com/navikt/deployment/hookd/pkg/auth.TemplateLocation=/app/templates/ -X github.com/navikt/deployment/hookd/pkg/auth.StaticAssetsLocation=/app/assets/

.PHONY: all proto hookd deployd token-generator alpine test docker upload

all: hookd deployd token-generator

proto:
	wget -O deployment.proto https://raw.githubusercontent.com/navikt/protos/master/deployment/deployment.proto
	$(PROTOC) --plugin=$(PROTOC_GEN_GO) --go_out=. deployment.proto
	mv deployment.pb.go common/pkg/deployment/
	rm -f deployment.proto

hookd:
	go build -o hookd/hookd cmd/hookd/main.go

deployd:
	go build -o deployd/deployd cmd/deployd/main.go

token-generator:
	go build -o token-generator cmd/token-generator/*.go

alpine:
	go build -a -installsuffix cgo -ldflags "-s $(HOOKD_ALPINE_LDFLAGS)" -o hookd/hookd cmd/hookd/main.go
	go build -a -installsuffix cgo -o deployd/deployd cmd/deployd/main.go
	go build -a -installsuffix cgo -o token-generator cmd/token-generator/*.go

test:
	go test ./...

docker:
	docker build -t navikt/deployment:latest .

upload:
	docker push navikt/deployment:latest
