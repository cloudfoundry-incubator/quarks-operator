#!/usr/bin/env make

all: tools build test

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

.PHONY: build
build:
	bin/build

build-image:
	bin/build-image

build-helm:
	bin/build-helm

############ TEST TARGETS ############

test: vet lint test-unit test-integration test-integration-storage test-helm-e2e test-helm-e2e-storage test-cli-e2e test-integration-subcmds

test-unit:
	bin/test-unit

test-integration:
	bin/test-integration

test-cli-e2e:
	bin/test-cli-e2e

test-helm-e2e:
	bin/test-helm-e2e

test-helm-e2e-storage:
	bin/test-helm-e2e-storage

test-integration-storage:
	bin/test-integration-storage

test-integration-subcmds:
	bin/test-integration-subcmds
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

############ COVERAGE TARGETS ############

coverage:
	bin/coverage
