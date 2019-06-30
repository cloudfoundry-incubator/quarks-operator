FROM golang:1.12.2 AS build
COPY . /go/src/code.cloudfoundry.org/cf-operator
ARG GO111MODULE="on"
ENV GO111MODULE $GO111MODULE
RUN cd /go/src/code.cloudfoundry.org/cf-operator && \
    make build && \
    cp -p binaries/cf-operator /usr/local/bin/cf-operator

FROM cfcontainerization/cf-operator-base
COPY --from=build /usr/local/bin/cf-operator /usr/local/bin/cf-operator
ENTRYPOINT ["cf-operator"]
