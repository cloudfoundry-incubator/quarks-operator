ARG BASE_IMAGE=registry.opensuse.org/cloud/platform/quarks/sle_15_sp1/quarks-operator-base:latest


################################################################################
FROM golang:1.14.7 AS build
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
COPY --from=build /usr/local/bin/cf-operator /usr/local/bin/cf-operator
ENTRYPOINT ["/usr/bin/dumb-init", "--", "cf-operator"]
