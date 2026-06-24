FROM golang:1.26.2-alpine AS builder

ARG TARGET=./services/notification-service

WORKDIR /app

COPY go.work go.work.sum go.mod go.sum ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/subscription-api/go.mod services/subscription-api/go.sum ./services/subscription-api/
COPY services/scanner-service/go.mod services/scanner-service/go.sum ./services/scanner-service/
COPY services/notification-service/go.mod services/notification-service/go.sum ./services/notification-service/

RUN go work sync \
    && for dir in . pkg services/subscription-api services/scanner-service services/notification-service; do \
        (cd "$dir" && go mod download); \
    done

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/service ${TARGET}

FROM alpine:3.22

RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -S subber \
    && adduser -S -G subber subber

WORKDIR /app

COPY --from=builder /out/service ./service

USER subber

CMD ["./service"]
