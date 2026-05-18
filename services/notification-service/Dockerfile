FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o notification main.go

FROM alpine:latest

WORKDIR /root/
COPY --from=builder /app/notification .

EXPOSE 8005
CMD ["./notification"]