#!/usr/bin/env make

MAKEFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MAKEFILE_DIR := $(patsubst %/,%,$(dir $(MAKEFILE_PATH)))
export PROJECT ?= quarks-operator
export QUARKS_UTILS ?= tools/quarks-utils
export GROUP_VERSIONS ?= boshdeployment:v1alpha1

all: tools build test

setup:
	# installs ginkgo, counterfeiter, linters, ...
	# includes bin/tools
	bin/dev-tools

.PHONY: tools
tools:
	bin/tools

up:
	bin/up

############ LINTER TARGETS ############

lint: tools
	$(QUARKS_UTILS)/bin/lint

check-scripts:
	bin/check-scripts

staticcheck:
	staticcheck ./...

vet:
	go list ./... | xargs go vet

############ BUILD TARGETS ############

.PHONY: build
build:
	bin/build

build-image:
	bin/build-image

build-helm:
	bin/build-helm

############ TEST TARGETS ############

test: lint test-unit test-integration test-integration-storage test-helm-e2e test-helm-e2e-storage test-cli-e2e test-integration-subcmds

test-unit: tools
	$(QUARKS_UTILS)/bin/test-unit

test-integration: tools
	$(QUARKS_UTILS)/bin/test-integration

test-cli-e2e: tools
	$(QUARKS_UTILS)/bin/test-cli-e2e

test-helm-e2e: tools build-helm
	$(QUARKS_UTILS)/bin/test-helm-e2e

test-helm-e2e-storage: tools build-helm
	bin/test-helm-e2e-storage

test-helm-e2e-upgrade: tools build-helm
	bin/test-helm-e2e-upgrade

test-integration-storage: tools
	INTEGRATION_SUITE=storage $(QUARKS_UTILS)/bin/test-integration

test-integration-subcmds: tools
	INTEGRATION_SUITE=util $(QUARKS_UTILS)/bin/test-integration

############ GENERATE TARGETS ############

generate: gen-kube gen-fakes

gen-kube: tools
	$(QUARKS_UTILS)/bin/gen-kube

gen-fakes: setup
	bin/gen-fakes

gen-command-docs:
	go run cmd/gen-command-docs.go docs/commands/

gen-crd-docs:
	kubectl get crd boshdeployments.quarks.cloudfoundry.org -o yaml > docs/crds/quarks_v1alpha1_boshdeployment_crd.yaml
	kubectl get crd quarksstatefulsets.quarks.cloudfoundry.org -o yaml > docs/crds/quarks_v1alpha1_quarksstatefulset_crd.yaml

verify-gen-kube:
	bin/verify-gen-kube

############ COVERAGE TARGETS ############

coverage: tools
	$(QUARKS_UTILS)/bin/coverage
