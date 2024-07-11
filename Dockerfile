FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o app ./cmd

FROM alpine

WORKDIR /app

RUN apk add --no-cache tzdata
RUN addgroup --system alpinegroup && adduser --system alpineuser -g alpinegroup
RUN chown -R alpineuser:alpinegroup /app

USER alpineuser

COPY --from=builder /app/app .

ENTRYPOINT ["./app"]