FROM golang:1.22.1-alpine as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -o main ./cmd/main.go

FROM alpine:latest

COPY --from=builder /app/main /main
CMD ["./main"]
