ARG GO_VERSION=1.23.5
ARG TARGETARCH

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build

WORKDIR /src

RUN apt-get update && apt-get install -y build-essential git cmake

COPY . .

RUN rm -rf go-llama.cpp && \
    git clone https://github.com/go-skynet/go-llama.cpp.git && \
    cd go-llama.cpp && git submodule update --init --recursive

    RUN cd /src/go-llama.cpp/llama.cpp && \
    if [ -f common/common.h ] && [ ! -f common.h ]; then ln -s common/common.h common.h; fi && \
    if [ -f common/sampling.h ] && [ ! -f sampling.h ]; then ln -s common/sampling.h sampling.h; fi && \
    if [ -f common/log.h ] && [ ! -f log.h ]; then ln -s common/log.h log.h; fi && \
    if [ -f common/console.h ] && [ ! -f console.h ]; then ln -s common/console.h console.h; fi

ENV CGO_ENABLED=1
ENV CC=gcc
ENV CXX=g++
ENV CGO_CFLAGS="-I/src/go-llama.cpp/llama.cpp"
ENV CGO_LDFLAGS="-L/src/go-llama.cpp -lbinding -lstdc++ -lm -lpthread"

RUN cd go-llama.cpp && make libbinding.a

RUN go mod download
RUN go build -v -o /bin/discord-quotes-bot .

FROM debian:bullseye-slim

WORKDIR /app

COPY --from=build /bin/discord-quotes-bot /app/discord-quotes-bot
COPY --from=build /src/go-llama.cpp /app/go-llama.cpp

CMD ["/app/discord-quotes-bot"]

