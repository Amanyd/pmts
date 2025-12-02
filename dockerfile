FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .


RUN go build -o /bin/storage ./cmd/storage-service
RUN go build -o /bin/scraper ./cmd/scraper-service
RUN go build -o /bin/alert ./cmd/alert-service
RUN go build -o /bin/gateway ./cmd/api-gateway


FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /bin/storage /app/storage
COPY --from=builder /bin/scraper /app/scraper
COPY --from=builder /bin/alert /app/alert
COPY --from=builder /bin/gateway /app/gateway


CMD ["/app/storage"]