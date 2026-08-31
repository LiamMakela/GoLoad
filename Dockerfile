FROM golang:1.27-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build \
    -o /goload \
    ./cmd/goload


FROM alpine:latest

WORKDIR /app

COPY --from=build \
    /goload \
    /app/goload

COPY configs \
    /app/configs

EXPOSE 8080

CMD ["/app/goload"]