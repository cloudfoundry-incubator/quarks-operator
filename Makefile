#!/usr/bin/env make

MAKEFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MAKEFILE_DIR := $(patsubst %/,%,$(dir $(MAKEFILE_PATH)))
export PROJECT ?= quarks-operator
export QUARKS_UTILS ?= tools/quarks-utils
export GROUP_VERSIONS ?= boshdeployment:v1alpha1 quarksstatefulset:v1alpha1 quarkssecret:v1alpha1

all: tools build test

up:
	bin/up

vet:
	bin/vet

lint: tools
	$(QUARKS_UTILS)/bin/lint

.PHONY: tools
tools:
	bin/tools

check-scripts:
	bin/check-scripts

vendor:
	go mod vendor

staticcheck:
	staticcheck ./...

############ BUILD TARGETS ############

.PHONY: build build-container-run
build:
	bin/build

build-container-run:
	bin/build-container-run $(MAKEFILE_DIR)/binaries

build-image:
	bin/build-image

build-helm:
	bin/build-helm

############ TEST TARGETS ############

test: vet lint staticcheck test-unit test-integration test-integration-storage test-helm-e2e test-helm-e2e-storage test-cli-e2e test-integration-subcmds

test-unit: tools
	$(QUARKS_UTILS)/bin/test-unit

test-integration: tools
	$(QUARKS_UTILS)/bin/test-integration

test-cli-e2e: tools
	$(QUARKS_UTILS)/bin/test-cli-e2e

test-helm-e2e: tools
	$(QUARKS_UTILS)/bin/test-helm-e2e

test-helm-e2e-storage:
	bin/test-helm-e2e-storage

test-integration-storage:
	INTEGRATION_SUITE=storage $(QUARKS_UTILS)/bin/test-integration

test-integration-subcmds:
	INTEGRATION_SUITE=util $(QUARKS_UTILS)/bin/test-integration util
############ GENERATE TARGETS ############

generate: gen-kube gen-fakes

gen-kube: tools
	$(QUARKS_UTILS)/bin/gen-kube

gen-fakes:
	bin/gen-fakes

gen-command-docs:
	rm -f docs/commands/*
	go run cmd/gen-command-docs.go

gen-crd-docs:
	kubectl get crd boshdeployments.quarks.cloudfoundry.org -o yaml > docs/crds/quarks_v1alpha1_boshdeployment_crd.yaml
	kubectl get crd quarkssecrets.quarks.cloudfoundry.org -o yaml > docs/crds/quarks_v1alpha1_quarkssecret_crd.yaml
	kubectl get crd quarksstatefulsets.quarks.cloudfoundry.org -o yaml > docs/crds/quarks_v1alpha1_quarksstatefulset_crd.yaml

verify-gen-kube:
	bin/verify-gen-kube

############ COVERAGE TARGETS ############

coverage: tools
	$(QUARKS_UTILS)/bin/coverage
