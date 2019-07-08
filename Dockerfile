FROM golang:1.12.2 AS build
COPY . /go/src/code.cloudfoundry.org/cf-operator
ARG GO111MODULE="on"
ENV GO111MODULE $GO111MODULE
RUN cd /go/src/code.cloudfoundry.org/cf-operator && \
    make build && \
    cp -p binaries/cf-operator /usr/local/bin/cf-operator

FROM cfcontainerization/cf-operator-base@sha256:2bb233fea55317729ebba6636976459e56d3c6605eaf3c54774449e9507341a5
RUN groupadd -g 1000 vcap && \
    useradd -r -u 1000 -g vcap vcap
USER vcap
COPY --from=build /usr/local/bin/cf-operator /usr/local/bin/cf-operator
ENTRYPOINT ["cf-operator"]
