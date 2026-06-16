# syntax=docker/dockerfile:1

###############################################################################
# Build stage
###############################################################################
FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w \
      -X main.version=${VERSION} \
      -X main.commit=${COMMIT} \
      -X main.buildDate=${BUILD_DATE}" \
    -o /out/app .

###############################################################################
# Runtime stage — distroless, nonroot
###############################################################################
FROM gcr.io/distroless/static:nonroot

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown
LABEL org.opencontainers.image.title="keda-queue-worker" \
      org.opencontainers.image.description="RabbitMQ consumer/producer for KEDA queue-based scaling demo" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="https://github.com/bhuv01/keda-queue-worker" \
      org.opencontainers.image.licenses="MIT"

WORKDIR /
COPY --from=build /out/app /app
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app"]
