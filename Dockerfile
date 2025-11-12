FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
# Cross-compile to AMD64 - runs natively on ARM, outputs AMD64 binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .

# Use AMD64 runtime image (but this is tiny and doesn't matter much)
FROM --platform=linux/amd64 alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]
