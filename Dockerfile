# Build stage
FROM golang:1.27-bookworm AS builder

WORKDIR /app

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        git \
        build-essential && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

# Install repository analysis tools.
RUN go install honnef.co/go/tools/cmd/staticcheck@latest && \
    go install github.com/securego/gosec/v2/cmd/gosec@latest && \
    go install golang.org/x/vuln/cmd/govulncheck@latest

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /app/ai-code-review \
    ./cmd/server


# Runtime stage
FROM golang:1.27-bookworm

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        git \
        build-essential && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Application
COPY --from=builder /app/ai-code-review /app/ai-code-review

# Go analysis tools
COPY --from=builder /go/bin/staticcheck /usr/local/bin/staticcheck
COPY --from=builder /go/bin/gosec /usr/local/bin/gosec
COPY --from=builder /go/bin/govulncheck /usr/local/bin/govulncheck

EXPOSE 8080

ENTRYPOINT ["/app/ai-code-review"]