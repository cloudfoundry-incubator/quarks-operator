image:
	go build -o build/_output/bin/cf-operator cmd/manager/main.go
	docker build . -f build/Dockerfile -t cf-operator:latest

export WATCH_NAMESPACE ?= default
up:
	go run cmd/manager/main.go

generate:
	bash vendor/k8s.io/code-generator/generate-groups.sh deepcopy code.cloudfoundry.org/cf-operator/pkg/generated github.com/cloudfoundry-incubator/cf-operator/pkg/apis fissile:v1alpha1,
