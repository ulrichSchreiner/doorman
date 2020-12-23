FROM registry.gitlab.com/ulrichschreiner/base/debian:buster-slim

RUN apt -y update && \
    apt -y install ca-certificates curl tzdata && \
    rm -rf /var/lib/apt/lists/*

COPY bin/doorman /doorman
COPY webapp/dist /webapp/dist
RUN useradd --no-create-home --user-group --shell /bin/bash --home-dir /work --uid 1234 doorman
USER doorman

ENTRYPOINT ["/doorman"]