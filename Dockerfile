ARG BASE_IMAGE=registry.opensuse.org/cloud/platform/quarks/sle_15_sp1/quarks-operator-base:latest


################################################################################
FROM golang:1.16.3 AS containerrun
ARG GOPROXY
ENV GOPROXY $GOPROXY

RUN CGO_ENABLED=0  go install -ldflags="-s -w" code.cloudfoundry.org/quarks-container-run/cmd@v0.0.3
RUN mv /go/bin/cmd /usr/local/bin/container-run

################################################################################
FROM golang:1.16.3 AS build
ARG GOPROXY
ENV GOPROXY $GOPROXY

WORKDIR /go/src/code.cloudfoundry.org/quarks-operator
# First, download dependencies so we can cache this layer
COPY go.mod .
COPY go.sum .

# Copy the rest of the source code and build
COPY . .
RUN make build && \
    cp -p binaries/quarks-operator /usr/local/bin/quarks-operator

################################################################################
FROM $BASE_IMAGE
LABEL org.opencontainers.image.source https://github.com/cloudfoundry-incubator/quarks-operator
RUN groupadd -g 1000 vcap && \
    useradd -r -u 1000 -g vcap vcap
RUN cp /usr/sbin/dumb-init /usr/bin/dumb-init
USER 1000
COPY --from=containerrun /usr/local/bin/container-run /usr/local/bin/container-run
COPY --from=build /usr/local/bin/quarks-operator /usr/local/bin/quarks-operator
ENTRYPOINT ["/usr/bin/dumb-init", "--", "quarks-operator"]
