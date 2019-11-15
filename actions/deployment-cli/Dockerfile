FROM alpine

RUN apk --no-cache add ca-certificates curl xz tree

RUN curl -L https://github.com/navikt/deployment-cli/releases/download/v0.3.0/deployment-cli-v0.3.0-x86_64-unknown-linux-musl.xz | xz -d - > /deployment-cli; chmod +x /deployment-cli

ENV RUST_BACKTRACE=1

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
