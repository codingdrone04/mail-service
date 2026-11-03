FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o mail-service ./cmd/server

FROM alpine:3.24
WORKDIR /app
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /app/mail-service .
USER appuser
EXPOSE 8080
CMD ["./mail-service"]
