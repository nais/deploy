PROTOC = $(shell which protoc)
PROTOC_GEN_GO = $(shell which protoc-gen-go)
HOOKD_ALPINE_LDFLAGS := -X github.com/navikt/deployment/hookd/pkg/auth.TemplateLocation=/app/templates/ -X github.com/navikt/deployment/hookd/pkg/auth.StaticAssetsLocation=/app/assets/

.PHONY: all proto hookd deployd token-generator deploy alpine test docker upload

all: hookd deployd token-generator deploy

proto:
	wget -O deployment.proto https://raw.githubusercontent.com/navikt/protos/master/deployment/deployment.proto
	$(PROTOC) --plugin=$(PROTOC_GEN_GO) --go_out=. deployment.proto
	mv deployment.pb.go common/pkg/deployment/
	rm -f deployment.proto

hookd:
	go build -o bin/hookd cmd/hookd/main.go

deployd:
	go build -o bin/deployd cmd/deployd/main.go

token-generator:
	go build -o bin/token-generator cmd/token-generator/*.go

deploy:
	go build -o bin/deploy cmd/deploy/*.go

alpine:
	go build -a -installsuffix cgo -ldflags "-s $(HOOKD_ALPINE_LDFLAGS)" -o bin/hookd cmd/hookd/main.go
	go build -a -installsuffix cgo -o bin/deployd cmd/deployd/main.go
	go build -a -installsuffix cgo -o bin/token-generator cmd/token-generator/*.go
	go build -a -installsuffix cgo -o bin/deploy cmd/deploy/*.go

test:
	go test ./...

docker:
	docker build -t navikt/deployment:latest .

upload:
	docker push navikt/deployment:latest
