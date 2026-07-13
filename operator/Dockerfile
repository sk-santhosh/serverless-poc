# syntax=docker/dockerfile:1

# --- Build stage -------------------------------------------------------
# NOTE (deviation from spec): the spec suggested golang:1.22-alpine, but
# current k8s.io/client-go + sigs.k8s.io/controller-runtime releases require
# a newer Go toolchain (see go.mod's `go` directive). Bumped to 1.26-alpine
# to match what `go mod tidy` actually resolved against this repo's deps.
FROM golang:1.26-alpine AS builder
WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY main.go main.go
COPY api/ api/
COPY internal/ internal/

# CGO disabled + static linking so the binary runs standalone on the
# distroless base below (no libc present).
RUN CGO_ENABLED=0 GOOS=linux go build -a -o operator main.go

# --- Runtime stage -------------------------------------------------------
FROM gcr.io/distroless/static:nonroot AS runner
WORKDIR /
COPY --from=builder /workspace/operator .

# distroless/static:nonroot already runs as uid 65532; kept explicit here
# for clarity and to survive base-image changes.
USER 65532:65532

ENTRYPOINT ["/operator"]
