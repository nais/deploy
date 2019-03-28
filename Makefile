PROTOC = $(shell which protoc)
PROTOC_GEN_GO = $(shell which protoc-gen-go)
HOOKD_ALPINE_LDFLAGS := -X github.com/navikt/deployment/hookd/pkg/auth.TemplateLocation=/app/templates/

.PHONY: all docker upload proto hookd deployd

all: hookd deployd

proto:
	$(PROTOC) --plugin=$(PROTOC_GEN_GO) --go_out=. protobuf/deployment.proto
	mv protobuf/deployment.pb.go common/pkg/deployment/

hookd:
	cd hookd && go build -o hookd cmd/hookd/main.go

deployd:
	cd deployd && go build -o deployd cmd/deployd/main.go

alpine:
	cd hookd && go build -a -installsuffix cgo -ldflags "-s $(HOOKD_ALPINE_LDFLAGS)" -o hookd cmd/hookd/main.go
	cd deployd && go build -a -installsuffix cgo -o deployd cmd/deployd/main.go

test:
	go test ./...

docker:
	docker build -t navikt/deployment:latest .

upload:
	docker push navikt/deployment:latest
