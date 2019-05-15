#!/usr/bin/env make

all: tools test-unit test-integration test-e2e build

up:
	bin/up

vet:
	bin/vet

lint:
	bin/lint

tools:
	bin/tools

check-scripts:
	bin/check-scripts

############ BUILD TARGETS ############

build:
	bin/build

build-image:
	bin/build-image

build-helm:
	bin/build-helm

############ TEST TARGETS ############

test: vet lint test-unit test-integration test-e2e

test-unit:
	bin/test-unit

test-integration:
	bin/test-integration

test-e2e:
	bin/test-e2e

############ GENERATE TARGETS ############

generate: gen-kube gen-fakes

gen-kube:
	bin/gen-kube

gen-fakes:
	bin/gen-fakes

gen-command-docs:
	go run cmd/gen-command-docs.go

verify-gen-kube:
	bin/verify-gen-kube