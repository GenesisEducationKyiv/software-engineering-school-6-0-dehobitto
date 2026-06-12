FROM golang:1.26.2-alpine AS builder

ARG TARGET=./services/subscription-api

WORKDIR /app

COPY . .

RUN go work sync
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/service ${TARGET}

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/service ./service
COPY --from=builder /app/services/subscription-api/static ./static

CMD ["./service"]
