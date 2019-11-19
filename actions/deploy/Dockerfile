FROM docker.pkg.github.com/nais/deploy/deploy:v1
RUN apk add --no-cache curl
RUN curl -L -f https://github.com/mikefarah/yq/releases/download/2.4.1/yq_linux_amd64 > /usr/local/bin/yq && chmod +x /usr/local/bin/yq
COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
