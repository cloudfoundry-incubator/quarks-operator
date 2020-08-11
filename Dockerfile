FROM golang:1.13.15 AS containerrun
ARG GOPROXY
ENV GOPROXY $GOPROXY

ENV GO111MODULE on

RUN wget https://github.com/cloudfoundry-incubator/quarks-container-run/archive/v0.0.1.tar.gz -O - | tar xz
WORKDIR /go/quarks-container-run-0.0.1/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o "/usr/local/bin/container-run" ./cmd

FROM golang:1.13.3 AS build

ARG GOPROXY
ENV GOPROXY $GOPROXY
ARG GO111MODULE="on"
ENV GO111MODULE $GO111MODULE

WORKDIR /go/src/code.cloudfoundry.org/cf-operator
# First, download dependencies so we can cache this layer
COPY go.mod .
COPY go.sum .
RUN if [ "${GO111MODULE}" = "on" ]; then go mod download; fi

# Copy the rest of the source code and build
COPY . .
RUN make build && \
    cp -p binaries/cf-operator /usr/local/bin/cf-operator

FROM registry.opensuse.org/cloud/platform/quarks/sle_15_sp1/quarks-operator-base:latest
RUN groupadd -g 1000 vcap && \
    useradd -r -u 1000 -g vcap vcap
RUN cp /usr/sbin/dumb-init /usr/bin/dumb-init
USER 1000
COPY --from=build /usr/local/bin/cf-operator /usr/local/bin/cf-operator
COPY --from=containerrun /usr/local/bin/container-run /usr/local/bin/container-run
ENTRYPOINT ["/usr/bin/dumb-init", "--", "cf-operator"]
