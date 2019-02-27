FROM golang:1.12 AS build
COPY . /go/src/code.cloudfoundry.org/cf-operator
RUN cd /go/src/code.cloudfoundry.org/cf-operator && \
    GO111MODULE=on make build && \
    cp -p binaries/cf-operator /usr/local/bin/cf-operator

FROM opensuse/leap:15.0
RUN zypper -n in system-user-nobody
USER nobody
COPY --from=build /usr/local/bin/cf-operator /usr/local/bin/cf-operator
