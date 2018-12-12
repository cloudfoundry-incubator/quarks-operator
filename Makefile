all: test-unit build image

.PHONY: build
build:
	bin/build

image:
	bin/build-image

.PHONY: helm
helm:
	bin/build-helm

export CFO_NAMESPACE ?= default
up:
	kubectl apply -f deploy/helm/cf-operator/templates/fissile_v1alpha1_boshdeployment_crd.yaml
	kubectl apply -f deploy/helm/cf-operator/templates/fissile_v1alpha1_extendedstatefulset_crd.yaml
	@echo watching namespace ${CFO_NAMESPACE}
	go run cmd/cf-operator/main.go

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

test-e2e:
	bin/test-e2e

test: vet lint test-unit test-integration test-e2e

tools:
	bin/tools
