# --------------------------------------------
# CONFIGURATION
# --------------------------------------------

# Build-time variables for version info
TAG            ?= latest
VERSION        ?= dev
GIT_COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE     ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# --------------------------------------------
# BUILD COMMANDS
# --------------------------------------------

## Build all
build: build-frontend build-backend

## Build frontend Docker image
build-frontend:
	docker build -t frontend:$(TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		./frontend

## Build backend Docker image
build-backend:
	docker build -t backend:$(TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		./backend

