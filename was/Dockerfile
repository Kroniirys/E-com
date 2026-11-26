FROM golang:1.27.4-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -trimpath -o /out/seed ./cmd/seed && \
    CGO_ENABLED=0 go build -trimpath -o /out/healthcheck ./cmd/healthcheck && \
    CGO_ENABLED=0 go build -trimpath -o /out/events ./cmd/events

FROM debian:bookworm-slim AS runtime
WORKDIR /app
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/ /app/
USER 65532:65532
EXPOSE 8080
CMD ["/app/api"]
