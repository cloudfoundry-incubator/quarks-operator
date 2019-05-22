FROM golang:1.12.2 AS build
COPY . /go/src/code.cloudfoundry.org/cf-operator
ARG GO111MODULE="on"
ENV GO111MODULE $GO111MODULE
ARG GIT_ROOT="/go/src/code.cloudfoundry.org/cf-operator"
ENV GIT_ROOT $GIT_ROOT
ARG ARTIFACT_VERSION
ENV ARTIFACT_VERSION $ARTIFACT_VERSION
RUN cd /go/src/code.cloudfoundry.org/cf-operator && \
    make build && \
    cp -p binaries/cf-operator /usr/local/bin/cf-operator

FROM opensuse/leap:15.1
RUN zypper -n in system-user-nobody ruby
RUN gem install bosh-template
USER nobody
COPY --from=build /usr/local/bin/cf-operator /usr/local/bin/cf-operator
ENTRYPOINT ["cf-operator"]
