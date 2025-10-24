ARG GO_VERSION=1.23.5

FROM golang:${GO_VERSION} AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -v -o /bin/discord-quotes-bot .

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    jq \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /bin/discord-quotes-bot /app/discord-quotes-bot
COPY init-model.sh /app/init-model.sh

RUN sed -i 's/\r$//' /app/init-model.sh && \
    chmod +x /app/init-model.sh

CMD ["/app/init-model.sh"]
