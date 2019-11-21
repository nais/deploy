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
RUN apk add --no-cache ca-certificates git curl
RUN curl -L -f https://github.com/mikefarah/yq/releases/download/2.4.1/yq_linux_amd64 > /usr/local/bin/yq && chmod +x /usr/local/bin/yq
WORKDIR /app
COPY --from=builder /src/bin/hookd /app/hookd
COPY --from=builder /src/bin/deployd /app/deployd
COPY --from=builder /src/bin/token-generator /app/token-generator
COPY --from=builder /src/bin/deploy /app/deploy
COPY --from=builder /src/hookd/templates /app/templates
COPY --from=builder /src/hookd/assets /app/assets
