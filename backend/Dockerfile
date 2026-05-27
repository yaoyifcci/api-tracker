FROM --platform=linux/amd64 golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o aitrace-server ./cmd/server/main.go

FROM --platform=linux/amd64 alpine:3.20
WORKDIR /app
COPY --from=builder /app/aitrace-server .
EXPOSE 8080
CMD ["./aitrace-server"]
