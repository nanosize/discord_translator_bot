FROM golang:1.27-alpine AS build

WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /discord-translator-bot .

FROM scratch
WORKDIR /app
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /discord-translator-bot /discord-translator-bot
COPY --chown=65534:65534 config.example.yml /data/config.yml

ENV CONFIG_FILE=/data/config.yml
USER 65534:65534
ENTRYPOINT ["/discord-translator-bot"]
