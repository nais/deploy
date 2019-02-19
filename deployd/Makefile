all:
	go build

docker:
	docker build -t navikt/deployd:latest .

upload:
	docker push navikt/deployd:latest
