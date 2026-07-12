# syntax=docker/dockerfile:1

FROM debian:bookworm-slim

# Make the buildx platform args available in this stage.
# Do not provide defaults here; defaults cause buildx to leave the
# values at their default instead of injecting the per-platform values.
ARG TARGETOS
ARG TARGETARCH

RUN groupadd --system --gid 10001 scaffold && \
    useradd --system --uid 10001 --gid scaffold --home-dir /data --shell /sbin/nologin scaffold

WORKDIR /app

# Install runtime tooling used by scenarios. Keep as root; we'll switch to the
# scaffold user after the binary is copied.
RUN apt-get update && \
    apt-get install -y ca-certificates curl golang-go unzip && \
    rm -rf /var/lib/apt/lists/*

ENV BUN_INSTALL=/usr/local
RUN curl -fsSL https://bun.sh/install | bash

COPY bin/${TARGETOS}/${TARGETARCH}/scaffold-bench-go /app/scaffold-bench-go

RUN mkdir -p /data && chown -R scaffold:scaffold /data /app

USER scaffold:scaffold

ENV BENCH_HTTP_ADDR=:8080
ENV BENCH_DB_PATH=/data/scaffold-bench.db
ENV BENCH_DATA_DIR=/data

EXPOSE 8080

VOLUME ["/data"]

ENTRYPOINT ["/app/scaffold-bench-go"]
