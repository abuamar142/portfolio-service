FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/portfolio-service ./cmd/portfolio-service

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/portfolio-service .
COPY --from=builder /app/cmd/portfolio-service/migrations ./migrations
EXPOSE 8084
CMD ["./portfolio-service"]
