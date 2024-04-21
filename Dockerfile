FROM golang:1.22.1 as builder

ENV CGO_ENABLED=1

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -o main ./cmd/main.go

FROM golang:1.22.1

WORKDIR /app
COPY --from=builder /app/main /app/main
COPY --from=builder /app/static /app/static

CMD [ "./main" ]
