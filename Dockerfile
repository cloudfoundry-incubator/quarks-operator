FROM alpine AS dumb-init
ADD https://github.com/Yelp/dumb-init/releases/download/v1.2.2/dumb-init_1.2.2_amd64 /usr/bin/dumb-init
RUN chmod +x /usr/bin/dumb-init

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
RUN ./bin/build-container-run /usr/local/bin

FROM cfcontainerization/cf-operator-base@sha256:2bb233fea55317729ebba6636976459e56d3c6605eaf3c54774449e9507341a5
RUN groupadd -g 1000 vcap && \
    useradd -r -u 1000 -g vcap vcap
USER vcap
COPY --from=dumb-init /usr/bin/dumb-init /usr/bin/dumb-init
COPY --from=build /usr/local/bin/cf-operator /usr/local/bin/cf-operator
COPY --from=build /usr/local/bin/container-run /usr/local/bin/container-run
ENTRYPOINT ["/usr/bin/dumb-init", "--", "cf-operator"]
