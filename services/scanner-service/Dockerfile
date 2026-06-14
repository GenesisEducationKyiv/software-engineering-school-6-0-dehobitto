FROM golang:1.26.2-alpine AS builder

ARG TARGET=./services/scanner-service

WORKDIR /app

COPY . .

RUN go work sync
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/service ${TARGET}

FROM alpine:3.22

RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -S subber \
    && adduser -S -G subber subber

WORKDIR /app

COPY --from=builder /out/service ./service

USER subber

CMD ["./service"]
