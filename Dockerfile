FROM golang:1.12 AS build
COPY . /go/src/code.cloudfoundry.org/cf-operator
ARG GO111MODULE="on"
ENV GO111MODULE $GO111MODULE
RUN cd /go/src/code.cloudfoundry.org/cf-operator && \
    make build && \
    cp -p binaries/cf-operator /usr/local/bin/cf-operator

FROM opensuse/leap:15.0
RUN zypper -n in system-user-nobody ruby
RUN gem install bosh-template
USER nobody
COPY --from=build /usr/local/bin/cf-operator /usr/local/bin/cf-operator
ENTRYPOINT ["cf-operator"]
