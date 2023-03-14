FROM golang:1.20-alpine as builder

RUN apk add --no-cache git make curl
ENV GOOS=linux
ENV CGO_ENABLED=0

# this used to live in the makefile, you may want to install kubebuiler manually
RUN curl -L https://github.com/kubernetes-sigs/kubebuilder/releases/download/v2.3.1/kubebuilder_2.3.1_linux_amd64.tar.gz | tar -xz -C /tmp/ && mv /tmp/kubebuilder_2.3.1_linux_amd64 /usr/local/kubebuilder


WORKDIR /src

# Copy dependency info
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy rest
COPY . .

RUN make test
RUN make alpine

FROM scratch
COPY --from=builder /src/bin/hookd /hookd
COPY --from=builder /src/bin/deployd /deployd
COPY --from=builder /src/bin/deploy /deploy
