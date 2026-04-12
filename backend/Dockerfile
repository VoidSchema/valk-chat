# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy all source files (needed for go mod tidy to resolve imports)
COPY . .

# Tidy modules and download dependencies
RUN go mod tidy && go mod download

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server .

# Runtime stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]
