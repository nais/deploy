FROM golang:1.12-alpine as builder
RUN apk add --no-cache git
ENV GOOS=linux
ENV CGO_ENABLED=0
ENV GO111MODULE=on
COPY . /src
WORKDIR /src/deployd
RUN rm -f ../go.sum
RUN go get
RUN go test ./...
RUN go build -a -installsuffix cgo -o deployd

FROM alpine:3.5
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /src/deployd/deployd /app/deployd
CMD ["/app/deployd"]
