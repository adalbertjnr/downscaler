FROM golang:alpine as builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o app

FROM alpine
WORKDIR /app
RUN addgroup --system alpinegroup && adduser --system alpineuser -g alpinegroup
RUN chown -R alpineuser:alpinegroup /app
USER alpineuser
COPY --from=builder /app .
EXPOSE 3000
ENTRYPOINT ["./app"]