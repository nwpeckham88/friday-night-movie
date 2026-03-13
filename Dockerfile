FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN go build -o /app/server main.go

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/server .
COPY frontend/ ./frontend/
RUN mkdir -p /app/data

EXPOSE 8080
ENTRYPOINT ["/app/server"]
