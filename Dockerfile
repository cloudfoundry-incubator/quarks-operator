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

FROM cfcontainerization/cf-operator-base@sha256:6495dd2e427e716fcdf1153dbca57e5ac6b1c68ca34c6ebb23be46d186fc2474
RUN groupadd -g 1000 vcap && \
    useradd -r -u 1000 -g vcap vcap
RUN cp /usr/sbin/dumb-init /usr/bin/dumb-init
USER vcap
COPY --from=build /usr/local/bin/cf-operator /usr/local/bin/cf-operator
COPY --from=build /usr/local/bin/container-run /usr/local/bin/container-run
ENTRYPOINT ["/usr/bin/dumb-init", "--", "cf-operator"]
