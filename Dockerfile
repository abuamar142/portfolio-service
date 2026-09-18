FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/quote-service ./cmd/quote-service

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/quote-service .
COPY --from=builder /app/cmd/quote-service/migrations ./migrations
EXPOSE 8084
CMD ["./quote-service"]
