// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"context"
	"sync"

	manifesta "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FakeDesiredManifest struct {
	DesiredManifestStub        func(context.Context, string, string) (*manifesta.Manifest, error)
	desiredManifestMutex       sync.RWMutex
	desiredManifestArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 string
	}
	desiredManifestReturns struct {
		result1 *manifesta.Manifest
		result2 error
	}
	desiredManifestReturnsOnCall map[int]struct {
		result1 *manifesta.Manifest
		result2 error
	}
	InjectClientStub        func(client.Client)
	injectClientMutex       sync.RWMutex
	injectClientArgsForCall []struct {
		arg1 client.Client
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeDesiredManifest) DesiredManifest(arg1 context.Context, arg2 string, arg3 string) (*manifesta.Manifest, error) {
	fake.desiredManifestMutex.Lock()
	ret, specificReturn := fake.desiredManifestReturnsOnCall[len(fake.desiredManifestArgsForCall)]
	fake.desiredManifestArgsForCall = append(fake.desiredManifestArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 string
	}{arg1, arg2, arg3})
	fake.recordInvocation("DesiredManifest", []interface{}{arg1, arg2, arg3})
	fake.desiredManifestMutex.Unlock()
	if fake.DesiredManifestStub != nil {
		return fake.DesiredManifestStub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.desiredManifestReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeDesiredManifest) DesiredManifestCallCount() int {
	fake.desiredManifestMutex.RLock()
	defer fake.desiredManifestMutex.RUnlock()
	return len(fake.desiredManifestArgsForCall)
}

func (fake *FakeDesiredManifest) DesiredManifestCalls(stub func(context.Context, string, string) (*manifesta.Manifest, error)) {
	fake.desiredManifestMutex.Lock()
	defer fake.desiredManifestMutex.Unlock()
	fake.DesiredManifestStub = stub
}

func (fake *FakeDesiredManifest) DesiredManifestArgsForCall(i int) (context.Context, string, string) {
	fake.desiredManifestMutex.RLock()
	defer fake.desiredManifestMutex.RUnlock()
	argsForCall := fake.desiredManifestArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeDesiredManifest) DesiredManifestReturns(result1 *manifesta.Manifest, result2 error) {
	fake.desiredManifestMutex.Lock()
	defer fake.desiredManifestMutex.Unlock()
	fake.DesiredManifestStub = nil
	fake.desiredManifestReturns = struct {
		result1 *manifesta.Manifest
		result2 error
	}{result1, result2}
}

func (fake *FakeDesiredManifest) DesiredManifestReturnsOnCall(i int, result1 *manifesta.Manifest, result2 error) {
	fake.desiredManifestMutex.Lock()
	defer fake.desiredManifestMutex.Unlock()
	fake.DesiredManifestStub = nil
	if fake.desiredManifestReturnsOnCall == nil {
		fake.desiredManifestReturnsOnCall = make(map[int]struct {
			result1 *manifesta.Manifest
			result2 error
		})
	}
	fake.desiredManifestReturnsOnCall[i] = struct {
		result1 *manifesta.Manifest
		result2 error
	}{result1, result2}
}

func (fake *FakeDesiredManifest) InjectClient(arg1 client.Client) {
	fake.injectClientMutex.Lock()
	fake.injectClientArgsForCall = append(fake.injectClientArgsForCall, struct {
		arg1 client.Client
	}{arg1})
	fake.recordInvocation("InjectClient", []interface{}{arg1})
	fake.injectClientMutex.Unlock()
	if fake.InjectClientStub != nil {
		fake.InjectClientStub(arg1)
	}
}

func (fake *FakeDesiredManifest) InjectClientCallCount() int {
	fake.injectClientMutex.RLock()
	defer fake.injectClientMutex.RUnlock()
	return len(fake.injectClientArgsForCall)
}

func (fake *FakeDesiredManifest) InjectClientCalls(stub func(client.Client)) {
	fake.injectClientMutex.Lock()
	defer fake.injectClientMutex.Unlock()
	fake.InjectClientStub = stub
}

func (fake *FakeDesiredManifest) InjectClientArgsForCall(i int) client.Client {
	fake.injectClientMutex.RLock()
	defer fake.injectClientMutex.RUnlock()
	argsForCall := fake.injectClientArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeDesiredManifest) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.desiredManifestMutex.RLock()
	defer fake.desiredManifestMutex.RUnlock()
	fake.injectClientMutex.RLock()
	defer fake.injectClientMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeDesiredManifest) recordInvocation(key string, args []interface{}) {
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

var _ manifest.DesiredManifest = new(FakeDesiredManifest)
