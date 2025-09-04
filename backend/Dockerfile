## --- Estágio 1: Builder ---
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Dependências em cache
COPY go.mod go.sum ./
RUN go mod download

# Código
COPY . .

# Build estático, pequeno e reprodutível
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w" \
    -o /app/main .

## --- Estágio 2: Final ---
FROM alpine:3.20

WORKDIR /app

# Certificados e fuso horário para HTTPS e logs corretos
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S app -G app

COPY --from=builder /app/main /app/main

# Executar como não-root
USER app

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/main"]


