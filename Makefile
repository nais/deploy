PROTOC = $(shell which protoc)
PROTOC_GEN_GO = $(shell which protoc-gen-go)

.PHONY: all docker upload proto hookd hookd-docker hookd-upload deployd deployd-docker deployd-upload

all: hookd deployd

docker: hookd-docker deployd-docker

upload: hookd-upload deployd-upload

proto:
	$(PROTOC) --plugin=$(PROTOC_GEN_GO) --go_out=. protobuf/deployment.proto
	mv protobuf/deployment.pb.go common/pkg/deployment/

hookd:
	cd hookd && go build

hookd-docker:
	docker build -t navikt/hookd:latest -f docker/hookd/Dockerfile .

hookd-upload:
	docker push navikt/hookd:latest

deployd:
	cd deployd && go build

deployd-docker:
	docker build -t navikt/deployd:latest -f docker/deployd/Dockerfile .

deployd-upload:
	docker push navikt/deployd:latest
