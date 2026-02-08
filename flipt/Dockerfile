FROM golang:alpine AS builder
WORKDIR /build

COPY flipt/scripts/generate-acl-data.go .
COPY flipt/scripts/lint-flags.go .

RUN go mod init flipt-tools && go mod tidy \
    && go build -o generate-acl-data generate-acl-data.go \
    && go build -o lint-flags lint-flags.go

FROM ghcr.io/flipt-io/flipt:v2.5.0
USER root
RUN apk upgrade --no-cache && apk add --no-cache git

COPY --from=builder /build/generate-acl-data /usr/local/bin/generate-acl-data
COPY --from=builder /build/lint-flags /usr/local/bin/lint-flags

COPY flipt/scripts/entrypoint.sh /usr/local/bin/entrypoint
COPY flipt/policies/ /etc/flipt/policies/
COPY flipt/config/ /etc/flipt/config/
COPY flags/ /var/opt/flipt/repo/flags/

RUN generate-acl-data /var/opt/flipt/repo/flags /var/opt/flipt/acl-data.json

USER flipt

ENTRYPOINT ["entrypoint"]
