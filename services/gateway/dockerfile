# 1. Build Stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway .

# 2. Run Stage (Distroless/Alpine)
FROM alpine:latest

# Create a non-root user
RUN adduser -D -u 1001 gatewayuser

# Copy in the binary
COPY --from=builder --chown=gatewayuser:gatewayuser /app/gateway /app/gateway

# Switch to the non-root user
USER 1001

# Document 8080 as the default port 
EXPOSE 8080

CMD ["/app/gateway"]