FROM golang:1.26-alpine AS builder
WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o api-tracker-server ./cmd/server/main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/api-tracker-server .
EXPOSE 8080
CMD ["./api-tracker-server"]
