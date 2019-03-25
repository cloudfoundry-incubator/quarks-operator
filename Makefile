all: test-unit build image

.PHONY: build
build:
	bin/build

image:
	bin/build-image

image-nobuild:
	bin/build-nobuild-image

.PHONY: helm
helm:
	bin/build-helm

export CFO_NAMESPACE ?= default
up:
	bin/up

up-nobuild:
	bin/up-nobuild

gen-kube:
	bin/gen-kube

gen-fakes:
	bin/gen-fakes

verify-gen-kube:
	bin/verify-gen-kube

generate: gen-kube gen-fakes

vet:
	bin/vet

lint:
	bin/lint

test-unit:
	bin/test-unit

test-integration:
	bin/test-integration

test-integration-nobuild: build image-nobuild
	bin/test-integration

test-e2e:
	bin/test-e2e

test: vet lint test-unit test-integration test-e2e

tools:
	bin/tools

check-scripts:
	bin/check-scripts
