# Multi-stage build: frontend (Node, build-time only) -> backend (Go) -> minimal runtime image.
# Build from the repo root: docker build -f backend/Dockerfile -t k8s-workload-analyzer:local .

FROM node:24-alpine AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-alpine AS backend-build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# CGO_ENABLED=0: a static binary, so the final image needs no libc at all.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/analyzer ./cmd/analyzer

# distroless static: no shell, no package manager -- nothing to exploit beyond the binary itself.
# Runs as the image's built-in non-root "nonroot" user (uid 65532), matching the least-privilege
# posture already applied to the RBAC (D-007) and the demo/ fixtures.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend-build /out/analyzer ./analyzer
COPY --from=frontend-build /src/frontend/dist ./frontend/dist

ENV STATIC_DIR=/app/frontend/dist
ENV LISTEN_ADDR=0.0.0.0:8080

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/analyzer"]
