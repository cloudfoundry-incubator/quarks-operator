// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
)

type FakeResolver struct {
	WithOpsManifestStub        func(*v1alpha1.BOSHDeployment, string) (*manifest.Manifest, []string, error)
	withOpsManifestMutex       sync.RWMutex
	withOpsManifestArgsForCall []struct {
		arg1 *v1alpha1.BOSHDeployment
		arg2 string
	}
	withOpsManifestReturns struct {
		result1 *manifest.Manifest
		result2 []string
		result3 error
	}
	withOpsManifestReturnsOnCall map[int]struct {
		result1 *manifest.Manifest
		result2 []string
		result3 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeResolver) WithOpsManifest(arg1 *v1alpha1.BOSHDeployment, arg2 string) (*manifest.Manifest, []string, error) {
	fake.withOpsManifestMutex.Lock()
	ret, specificReturn := fake.withOpsManifestReturnsOnCall[len(fake.withOpsManifestArgsForCall)]
	fake.withOpsManifestArgsForCall = append(fake.withOpsManifestArgsForCall, struct {
		arg1 *v1alpha1.BOSHDeployment
		arg2 string
	}{arg1, arg2})
	fake.recordInvocation("WithOpsManifest", []interface{}{arg1, arg2})
	fake.withOpsManifestMutex.Unlock()
	if fake.WithOpsManifestStub != nil {
		return fake.WithOpsManifestStub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3
	}
	fakeReturns := fake.withOpsManifestReturns
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3
}

func (fake *FakeResolver) WithOpsManifestCallCount() int {
	fake.withOpsManifestMutex.RLock()
	defer fake.withOpsManifestMutex.RUnlock()
	return len(fake.withOpsManifestArgsForCall)
}

func (fake *FakeResolver) WithOpsManifestCalls(stub func(*v1alpha1.BOSHDeployment, string) (*manifest.Manifest, []string, error)) {
	fake.withOpsManifestMutex.Lock()
	defer fake.withOpsManifestMutex.Unlock()
	fake.WithOpsManifestStub = stub
}

func (fake *FakeResolver) WithOpsManifestArgsForCall(i int) (*v1alpha1.BOSHDeployment, string) {
	fake.withOpsManifestMutex.RLock()
	defer fake.withOpsManifestMutex.RUnlock()
	argsForCall := fake.withOpsManifestArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeResolver) WithOpsManifestReturns(result1 *manifest.Manifest, result2 []string, result3 error) {
	fake.withOpsManifestMutex.Lock()
	defer fake.withOpsManifestMutex.Unlock()
	fake.WithOpsManifestStub = nil
	fake.withOpsManifestReturns = struct {
		result1 *manifest.Manifest
		result2 []string
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeResolver) WithOpsManifestReturnsOnCall(i int, result1 *manifest.Manifest, result2 []string, result3 error) {
	fake.withOpsManifestMutex.Lock()
	defer fake.withOpsManifestMutex.Unlock()
	fake.WithOpsManifestStub = nil
	if fake.withOpsManifestReturnsOnCall == nil {
		fake.withOpsManifestReturnsOnCall = make(map[int]struct {
			result1 *manifest.Manifest
			result2 []string
			result3 error
		})
	}
	fake.withOpsManifestReturnsOnCall[i] = struct {
		result1 *manifest.Manifest
		result2 []string
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeResolver) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.withOpsManifestMutex.RLock()
	defer fake.withOpsManifestMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeResolver) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ boshdeployment.Resolver = new(FakeResolver)
