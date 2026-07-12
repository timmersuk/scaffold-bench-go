# syntax=docker/dockerfile:1

ARG TARGETOS=linux
ARG TARGETARCH=amd64

FROM debian:bookworm-slim

RUN groupadd --system --gid 10001 scaffold && \
    useradd --system --uid 10001 --gid scaffold --home-dir /data --shell /sbin/nologin scaffold

WORKDIR /app

COPY bin/${TARGETOS}/${TARGETARCH}/scaffold-bench-go /app/scaffold-bench-go

RUN mkdir -p /data && chown -R scaffold:scaffold /data /app

USER scaffold:scaffold

ENV BENCH_HTTP_ADDR=:8080
ENV BENCH_DB_PATH=/data/scaffold-bench.db
ENV BENCH_DATA_DIR=/data

EXPOSE 8080

VOLUME ["/data"]

ENTRYPOINT ["/app/scaffold-bench-go"]
