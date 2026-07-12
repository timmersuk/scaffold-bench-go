IMAGE ?= timmersuk/scaffold-bench-go
CI_BUILD_ID ?= local
DATA_DIR ?= $(CURDIR)/data
DOCKER ?= docker
DOCKER_BUILDX ?= docker buildx

# Architectures we want binaries for.
ARCHES := amd64 arm64
OSES := linux windows

.PHONY: frontend frontend-docker build-go-docker build-go-local test build \
        docker-build docker-run docker-login docker-push docker-buildx-push \
        compose-up compose-down

# ------------------------------------------------------------------
# 1️⃣ Build the frontend using a node container (pnpm is available there).
# ------------------------------------------------------------------
frontend-docker:
	@echo "Building frontend with node:20-bookworm…"
	@docker run --rm -e CI=true \
	    -v "$(CURDIR):/src" \
	    -w /src \
	    node:20-bookworm \
	    sh -c "make frontend"

# ------------------------------------------------------------------
# 2️⃣ Build the Go binaries using a golang container.
# ------------------------------------------------------------------
build-go-docker:
	@echo "Building Go binaries for all supported architectures in Docker…"
	docker run --rm \
	            -v "$(CURDIR):/src" \
	            -w /src \
	            golang:1.26-bookworm \
	            sh -c 'make build-go-local'

# ------------------------------------------------------------------
# 3️⃣ Public build targets that run the frontend and Go builds in Docker containers, or locally.
# ------------------------------------------------------------------
build-docker: frontend-docker build-go-docker
build: frontend build-go-local

# ------------------------------------------------------------------
# 4️⃣ Build the frontend locally.
# ------------------------------------------------------------------
frontend:
	@echo "Building frontend locally…"
	@corepack enable && corepack prepare pnpm@10.23.0 --activate && pnpm --dir frontend install --frozen-lockfile && pnpm --dir frontend build

# ------------------------------------------------------------------
# 5️⃣ Build the Go binaries for all supported architectures (cross‑compile) locally.
# ------------------------------------------------------------------
build-go-local: frontend
	@echo "Building Go binaries for all supported architectures…"
	@for os in $(OSES); do \
	    for arch in $(ARCHES); do \
	        mkdir -p bin/$$os/$$arch && \
	        GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "-s -w -X main.BuildID=$(CI_BUILD_ID)" -o bin/$$os/$$arch/scaffold-bench-go ./cmd/server; \
	    done; \
	done

# ------------------------------------------------------------------
# 6️⃣ Existing test target – still depends on local frontend files.
# ------------------------------------------------------------------
test: frontend
	go test ./...

dev:
	pnpm --dir frontend dev

run:
	go run ./cmd/server

docker-build:
	$(DOCKER) build -t $(IMAGE):$(CI_BUILD_ID) .

docker-run:
	$(DOCKER) run --rm \
		-p 8080:8080 \
		-v "$(DATA_DIR):/data" \
		-e BENCH_HTTP_ADDR=:8080 \
		-e BENCH_DB_PATH=/data/scaffold-bench.db \
		timmersuk/scaffold-bench-go:$(CI_BUILD_ID)

compose-up:
	$(DOCKER) compose up --build

compose-down:
	$(DOCKER) compose down
