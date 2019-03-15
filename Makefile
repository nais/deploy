PROTOC = $(shell which protoc)
PROTOC_GEN_GO = $(shell which protoc-gen-go)

.PHONY: all docker upload proto hookd deployd

all: hookd deployd

proto:
	$(PROTOC) --plugin=$(PROTOC_GEN_GO) --go_out=. protobuf/deployment.proto
	mv protobuf/deployment.pb.go common/pkg/deployment/

hookd:
	cd hookd && go build

deployd:
	cd deployd && go build

docker:
	docker build -t navikt/deployment:latest .

upload:
	docker push navikt/deployment:latest
