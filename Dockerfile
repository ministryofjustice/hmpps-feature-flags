FROM ghcr.io/flipt-io/flipt:v2.5.0
USER root
RUN apk upgrade --no-cache
USER flipt

CMD ["flipt", "server"]
