ARG GO_VERSION=1.23.5
ARG TARGETARCH

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build

WORKDIR /src

RUN apt-get update && apt-get install -y build-essential git cmake

COPY . .

RUN rm -rf go-llama.cpp && \
    git clone https://github.com/go-skynet/go-llama.cpp.git && \
    cd go-llama.cpp && \
    git submodule update --init --recursive && \
    cd llama.cpp && \
    git fetch origin && \
    git checkout master && \
    git pull origin master

RUN cd /src/go-llama.cpp && \
    sed -i 's|#include "common.h"|#include "common/common.h"|g' binding.cpp && \
    sed -i 's|#include "sampling.h"|#include "common/sampling.h"|g' binding.cpp && \
    sed -i 's|#include "log.h"|#include "common/log.h"|g' binding.cpp && \
    sed -i 's|#include "console.h"|#include "common/console.h"|g' binding.cpp

ENV CGO_ENABLED=1
ENV CC=gcc
ENV CXX=g++
ENV CGO_CFLAGS="-I/src/go-llama.cpp/llama.cpp -I/src/go-llama.cpp/llama.cpp/common"
ENV CGO_LDFLAGS="-L/src/go-llama.cpp -lbinding -lstdc++ -lm -lpthread"

RUN cd go-llama.cpp && make libbinding.a

RUN go mod edit -replace github.com/go-skynet/go-llama.cpp=/src/go-llama.cpp

RUN go mod download
RUN go build -v -o /bin/discord-quotes-bot .

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y \
    libstdc++6 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /bin/discord-quotes-bot /app/discord-quotes-bot
COPY --from=build /src/go-llama.cpp /app/go-llama.cpp

CMD ["/app/discord-quotes-bot"]

