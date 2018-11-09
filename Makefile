all: test-unit build image

.PHONY: build
build:
	bin/build

image: build
	bin/build-image

export WATCH_NAMESPACE ?= default
up:
	kubectl apply -f deploy/crds/fissile_v1alpha1_boshdeployment_crd.yaml
	@echo watching namespace ${WATCH_NAMESPACE}
	go run cmd/manager/main.go

generate:
	bash ${GOPATH}/src/k8s.io/code-generator/generate-groups.sh deepcopy code.cloudfoundry.org/cf-operator/pkg/generated github.com/cloudfoundry-incubator/cf-operator/pkg/apis fissile:v1alpha1,
	client-gen -h /dev/null --clientset-name versioned --input-base code.cloudfoundry.org/cf-operator --input pkg/apis/fissile/v1alpha1 --output-package code.cloudfoundry.org/cf-operator/pkg/client/clientset
	bin/gen-fakes

test-unit:
	bin/test

test-integration:
	bin/test-integration

test: test-unit test-integration

publish: image
	bin/publish
