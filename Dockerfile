FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o orchestrator main.go

FROM alpine:latest
WORKDIR /root/app
COPY --from=builder /app/orchestrator .
EXPOSE 8080
CMD ["./orchestrator"]