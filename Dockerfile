FROM golang:1.12.2 AS build
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

FROM cfcontainerization/cf-operator-base@sha256:2bb233fea55317729ebba6636976459e56d3c6605eaf3c54774449e9507341a5
RUN groupadd -g 1000 vcap && \
    useradd -r -u 1000 -g vcap vcap
USER vcap
COPY --from=build /usr/local/bin/cf-operator /usr/local/bin/cf-operator
ENTRYPOINT ["cf-operator"]
