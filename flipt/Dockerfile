FROM ghcr.io/flipt-io/flipt:v2.5.0
USER root
RUN apk upgrade --no-cache && apk add --no-cache git jq

COPY flipt/scripts/generate-acl-data.sh /usr/local/bin/generate-acl-data
COPY flipt/scripts/entrypoint.sh /usr/local/bin/entrypoint
COPY flipt/policies/ /etc/flipt/policies/
COPY flipt/config/ /etc/flipt/config/

USER flipt

ENTRYPOINT ["entrypoint"]
