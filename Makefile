all: test-unit build image

.PHONY: build
build:
	bin/build

image: build
	bin/build-image

helm:
	bin/build-helm

export CFO_NAMESPACE ?= default
up:
	kubectl apply -f deploy/helm/cf-operator/templates/fissile_v1alpha1_boshdeployment_crd.yaml
	@echo watching namespace ${CFO_NAMESPACE}
	go run main.go

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

test: vet lint test-unit test-integration

tools:
	bin/tools