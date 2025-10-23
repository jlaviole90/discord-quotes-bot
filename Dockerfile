ARG GO_VERSION=1.23.5
ARG TARGETARCH

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build

WORKDIR /src

RUN apt-get update && apt-get install -y build-essential git cmake

COPY . .

RUN rm -rf go-llama.cpp && \
    git clone https://github.com/go-skynet/go-llama.cpp.git && \
    cd go-llama.cpp && git submodule update --init --recursive

RUN cd go-llama.cpp && make libbinding.a

ENV CGO_ENABLED=1
ENV CC=gcc
ENV CXX=g++
ENV CGO_CFLAGS="-I/src/go-llama.cpp/llama.cpp"
ENV CGO_LDFLAGS="-L/src/go-llama.cpp -lbinding -lstdc++ -lm -lpthread"

RUN go mod download
RUN go build -v -o /bin/discord-quotes-bot .

FROM debian:bullseye-slim

WORKDIR /app

COPY --from=build /out/discord-quotes-bot /app/discord-quotes-bot
COPY --from=build /src/go-llama.cpp /app/go-llama.cpp

CMD ["app/discord-quotes-bot"]

