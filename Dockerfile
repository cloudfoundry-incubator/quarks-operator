FROM golang:1.11 AS build
COPY . /go/src/code.cloudfoundry.org/cf-operator
RUN cd /go/src/code.cloudfoundry.org/cf-operator && \
    make build && \
    cp -p binaries/cf-operator /usr/local/bin/cf-operator

FROM opensuse/leap:15.0

USER nobody

COPY --from=build /usr/local/bin/cf-operator /usr/local/bin/cf-operator


