# Use a small Go base image
FROM golang:1.23 AS builder

# Accept the target architecture from Buildx (e.g., amd64, arm64)
ARG TARGETARCH
ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

WORKDIR /app

# Copy module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the binary with static linking for the target architecture
# Pass version information as build-time variables
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -ldflags="-X main.Version=$VERSION -X main.GitCommit=$GIT_COMMIT -X main.BuildDate=$BUILD_DATE" \
    -o server .

# Use a minimal base image
FROM gcr.io/distroless/static:nonroot

# Re-declare ARGs after FROM to use them in this stage
ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

WORKDIR /app
COPY --from=builder /app/server /app/server

# Set version as environment variable for runtime access
ENV APP_VERSION=$VERSION
ENV GIT_COMMIT=$GIT_COMMIT
ENV BUILD_DATE=$BUILD_DATE

CMD ["/app/server"]
