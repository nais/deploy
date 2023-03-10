FROM golang:1.20-alpine as builder

RUN apk add --no-cache git make curl
ENV GOOS=linux
ENV CGO_ENABLED=0

WORKDIR /src

# Copy dependency info
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy rest
COPY . .

RUN make kubebuilder
RUN make test
RUN make alpine

FROM scratch
COPY --from=builder /src/bin/hookd /hookd
COPY --from=builder /src/bin/deployd /deployd
COPY --from=builder /src/bin/deploy /deploy
