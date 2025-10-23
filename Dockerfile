ARG GO_VERSION=1.23.5
ARG TARGETARCH

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build-cpp
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential cmake git wget pkg-config curl ca-certificates \
    g++ \
&& rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY . /src

RUN git submodule update --init --recursive || echo "no submodules or not using git data in build context"

ARG BINDING_DIR="/home/admin/discord-quotes-bot/go-llama.cpp"
RUN if [ -d "$BINDING_DIR" ]; then \
    cd $BINDING_DIR && make libbinding.a \
    else \
        echo "Binding dir not found at $BINDING_DIR"; exit 1; \
    fi

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build-go

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential g++ ca-certificates \
&& rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build-cpp /src /src

ARG BINDING_DIR=/src/thirdparty/go-llama.cpp
ENV CGO_ENABLED=1
ENV CC=gc/
ENV CXX=g++

ENV C_INCLUDE_PATH=$BINDING_DIR
ENV LIBRARY_PATH=$BINDING_DIR

RUN go env -w GOPRIVATE=*
# Download dependencies as a separate step to take advantage of Docker's caching.
# Leverage a cache mount to /go/pkg/mod/ to speed up subsequent builds.
# Leverage bind mounts to go.sum and go.mod to avoid having to copy them into
# the container.
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=1 go build -o /bin/discord-quotes-bot .

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libstdc++6 && rm -rf /var/lib/apt/lists/*
COPY --from=build-go /bin/discord-quotes-bot /usr/local/bin/discord-quotes-bot

WORKDIR /data
ENTRYPOINT ["/usr/local/bin/discord-quotes-bot"]

# syntax=docker/dockerfile:1

# Create a stage for building the application.
#FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
#WORKDIR /src


# This is the architecture you're building for, which is passed in by the builder.
# Placing it here allows the previous steps to be cached across architectures.

# Build the application.
# Leverage a cache mount to /go/pkg/mod/ to speed up subsequent builds.
# Leverage a bind mount to the current directory to avoid having to copy the
# source code into the container.
#RUN --mount=type=cache,target=/go/pkg/mod/ \
#    --mount=type=bind,target=. \
#    CGO_ENABLED=0 GOARCH=$TARGETARCH go build -o /bin/server .

################################################################################
# Create a new stage for running the application that contains the minimal
# runtime dependencies for the application. This often uses a different base
# image from the build stage where the necessary files are copied from the build
# stage.
#
# The example below uses the alpine image as the foundation for running the app.
# By specifying the "latest" tag, it will also use whatever happens to be the
# most recent version of that image when you build your Dockerfile. If
# reproducability is important, consider using a versioned tag
# (e.g., alpine:3.17.2) or SHA (e.g., alpine@sha256:c41ab5c992deb4fe7e5da09f67a8804a46bd0592bfdf0b1847dde0e0889d2bff).
#FROM alpine:latest AS final

# Install any runtime dependencies that are needed to run your application.
# Leverage a cache mount to /var/cache/apk/ to speed up subsequent builds.
#RUN --mount=type=cache,target=/var/cache/apk \
#    apk --update add \
#        ca-certificates \
#        tzdata \
#        && \
#        update-ca-certificates

# Create a non-privileged user that the app will run under.
# See https://docs.docker.com/go/dockerfile-user-best-practices/
#ARG UID=10001
#RUN adduser \
#    --disabled-password \
#    --gecos "" \
#    --home "/nonexistent" \
#    --shell "/sbin/nologin" \
#    --no-create-home \
#    --uid "${UID}" \
#    appuser
#USER appuser

# Copy the executable from the "build" stage.
#COPY --from=build /bin/server /bin/

# What the container should run when it is started.
#ENTRYPOINT [ "/bin/server" ]



