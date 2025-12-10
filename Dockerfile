# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
#COPY . .
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/ ./web/
# RUN ls -laR /app  # <-- Add this line to see everything
RUN CGO_ENABLED=0 GOOS=linux go build -o lorem-video ./cmd/server/

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ffmpeg ca-certificates
WORKDIR /app
COPY --from=builder /app/lorem-video .
COPY web/ ./web/
EXPOSE 3000
CMD ["./lorem-video"]
