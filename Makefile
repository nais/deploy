PROTOC = $(shell which protoc)
PROTOC_GEN_GO = $(shell which protoc-gen-go)

.PHONY: all proto hookd hookd-docker deployd deployd-docker upload

all: hookd deployd

proto:
	$(PROTOC) --plugin=$(PROTOC_GEN_GO) --go_out=. protobuf/deployment.proto
	mv protobuf/deployment.pb.go common/pkg/deployment/

hookd:
	cd hookd && go build

deployd:
	cd deployd && go build

hookd-docker:
	docker build -t navikt/hookd:latest -f docker/hookd/Dockerfile .

deployd-docker:
	docker build -t navikt/deployd:latest -f docker/deployd/Dockerfile .

upload:
	docker push navikt/hookd:latest
	docker push navikt/deployd:latest
