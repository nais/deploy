FROM golang:1.13-alpine as builder
RUN apk add --no-cache git make
ENV GOOS=linux
ENV CGO_ENABLED=0
ENV GO111MODULE=on
COPY . /src
WORKDIR /src
RUN rm -f go.sum
RUN make test
RUN make alpine

FROM alpine:3.10
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /src/hookd/hookd /app/hookd
COPY --from=builder /src/deployd/deployd /app/deployd
COPY --from=builder /src/token-generator /app/token-generator
COPY --from=builder /src/hookd/templates /app/templates
COPY --from=builder /src/hookd/assets /app/assets
