all: test-unit build image

.PHONY: build
build:
	bin/build

image: build
	bin/build-image

export WATCH_NAMESPACE ?= default
up:
	go run cmd/manager/main.go

generate:
	bash ${GOPATH}/src/k8s.io/code-generator/generate-groups.sh deepcopy code.cloudfoundry.org/cf-operator/pkg/generated github.com/cloudfoundry-incubator/cf-operator/pkg/apis fissile:v1alpha1,
	bin/gen-fakes

test-unit:
	bin/test

test-integration:
	bin/test-integration

test: test-unit test-integration

publish: image
	bin/publish
