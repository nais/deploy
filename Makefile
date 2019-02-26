PROTOC = $(shell which protoc)
PROTOC_GEN_GO = $(shell which protoc-gen-go)

.PHONY: proto hookd upload

proto:
	$(PROTOC) --plugin=$(PROTOC_GEN_GO) --go_out=. protobuf/deployment.proto
	mv protobuf/deployment.pb.go common/pkg/deployment/

hookd:
	docker build -t navikt/hookd:latest -f hookd.Dockerfile .

upload:
	docker push navikt/hookd:latest
