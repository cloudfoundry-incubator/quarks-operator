ARG BASE_IMAGE=registry.opensuse.org/cloud/platform/quarks/sle_15_sp1/quarks-operator-base:latest


################################################################################
FROM golang:1.13.15 AS containerrun
ARG GOPROXY
ENV GOPROXY $GOPROXY

ENV GO111MODULE on

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o "/usr/local/bin/container-run" code.cloudfoundry.org/quarks-container-run/cmd

################################################################################
FROM golang:1.15.1 AS build
ARG GOPROXY
ENV GOPROXY $GOPROXY

WORKDIR /go/src/code.cloudfoundry.org/quarks-operator
# First, download dependencies so we can cache this layer
COPY go.mod .
COPY go.sum .

# Copy the rest of the source code and build
COPY . .
RUN make build && \
    cp -p binaries/cf-operator /usr/local/bin/cf-operator

################################################################################
FROM $BASE_IMAGE
RUN groupadd -g 1000 vcap && \
    useradd -r -u 1000 -g vcap vcap
RUN cp /usr/sbin/dumb-init /usr/bin/dumb-init
USER 1000
COPY --from=containerrun /usr/local/bin/container-run /usr/local/bin/container-run
COPY --from=build /usr/local/bin/cf-operator /usr/local/bin/cf-operator
ENTRYPOINT ["/usr/bin/dumb-init", "--", "cf-operator"]
