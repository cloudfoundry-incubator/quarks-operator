// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
)

type FakeKubeConverter struct {
	BPMResourcesStub        func(string, manifest.DomainNameService, string, *manifest.InstanceGroup, converter.ReleaseImageProvider, bpm.Configs, string) (*converter.BPMResources, error)
	bPMResourcesMutex       sync.RWMutex
	bPMResourcesArgsForCall []struct {
		arg1 string
		arg2 manifest.DomainNameService
		arg3 string
		arg4 *manifest.InstanceGroup
		arg5 converter.ReleaseImageProvider
		arg6 bpm.Configs
		arg7 string
	}
	bPMResourcesReturns struct {
		result1 *converter.BPMResources
		result2 error
	}
	bPMResourcesReturnsOnCall map[int]struct {
		result1 *converter.BPMResources
		result2 error
	}
	VariablesStub        func(string, []manifest.Variable) ([]v1alpha1.ExtendedSecret, error)
	variablesMutex       sync.RWMutex
	variablesArgsForCall []struct {
		arg1 string
		arg2 []manifest.Variable
	}
	variablesReturns struct {
		result1 []v1alpha1.ExtendedSecret
		result2 error
	}
	variablesReturnsOnCall map[int]struct {
		result1 []v1alpha1.ExtendedSecret
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeKubeConverter) BPMResources(arg1 string, arg2 manifest.DomainNameService, arg3 string, arg4 *manifest.InstanceGroup, arg5 converter.ReleaseImageProvider, arg6 bpm.Configs, arg7 string) (*converter.BPMResources, error) {
	fake.bPMResourcesMutex.Lock()
	ret, specificReturn := fake.bPMResourcesReturnsOnCall[len(fake.bPMResourcesArgsForCall)]
	fake.bPMResourcesArgsForCall = append(fake.bPMResourcesArgsForCall, struct {
		arg1 string
		arg2 manifest.DomainNameService
		arg3 string
		arg4 *manifest.InstanceGroup
		arg5 converter.ReleaseImageProvider
		arg6 bpm.Configs
		arg7 string
	}{arg1, arg2, arg3, arg4, arg5, arg6, arg7})
	fake.recordInvocation("BPMResources", []interface{}{arg1, arg2, arg3, arg4, arg5, arg6, arg7})
	fake.bPMResourcesMutex.Unlock()
	if fake.BPMResourcesStub != nil {
		return fake.BPMResourcesStub(arg1, arg2, arg3, arg4, arg5, arg6, arg7)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.bPMResourcesReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeKubeConverter) BPMResourcesCallCount() int {
	fake.bPMResourcesMutex.RLock()
	defer fake.bPMResourcesMutex.RUnlock()
	return len(fake.bPMResourcesArgsForCall)
}

func (fake *FakeKubeConverter) BPMResourcesCalls(stub func(string, manifest.DomainNameService, string, *manifest.InstanceGroup, converter.ReleaseImageProvider, bpm.Configs, string) (*converter.BPMResources, error)) {
	fake.bPMResourcesMutex.Lock()
	defer fake.bPMResourcesMutex.Unlock()
	fake.BPMResourcesStub = stub
}

func (fake *FakeKubeConverter) BPMResourcesArgsForCall(i int) (string, manifest.DomainNameService, string, *manifest.InstanceGroup, converter.ReleaseImageProvider, bpm.Configs, string) {
	fake.bPMResourcesMutex.RLock()
	defer fake.bPMResourcesMutex.RUnlock()
	argsForCall := fake.bPMResourcesArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4, argsForCall.arg5, argsForCall.arg6, argsForCall.arg7
}

func (fake *FakeKubeConverter) BPMResourcesReturns(result1 *converter.BPMResources, result2 error) {
	fake.bPMResourcesMutex.Lock()
	defer fake.bPMResourcesMutex.Unlock()
	fake.BPMResourcesStub = nil
	fake.bPMResourcesReturns = struct {
		result1 *converter.BPMResources
		result2 error
	}{result1, result2}
}

func (fake *FakeKubeConverter) BPMResourcesReturnsOnCall(i int, result1 *converter.BPMResources, result2 error) {
	fake.bPMResourcesMutex.Lock()
	defer fake.bPMResourcesMutex.Unlock()
	fake.BPMResourcesStub = nil
	if fake.bPMResourcesReturnsOnCall == nil {
		fake.bPMResourcesReturnsOnCall = make(map[int]struct {
			result1 *converter.BPMResources
			result2 error
		})
	}
	fake.bPMResourcesReturnsOnCall[i] = struct {
		result1 *converter.BPMResources
		result2 error
	}{result1, result2}
}

func (fake *FakeKubeConverter) Variables(arg1 string, arg2 []manifest.Variable) ([]v1alpha1.ExtendedSecret, error) {
	var arg2Copy []manifest.Variable
	if arg2 != nil {
		arg2Copy = make([]manifest.Variable, len(arg2))
		copy(arg2Copy, arg2)
	}
	fake.variablesMutex.Lock()
	ret, specificReturn := fake.variablesReturnsOnCall[len(fake.variablesArgsForCall)]
	fake.variablesArgsForCall = append(fake.variablesArgsForCall, struct {
		arg1 string
		arg2 []manifest.Variable
	}{arg1, arg2Copy})
	fake.recordInvocation("Variables", []interface{}{arg1, arg2Copy})
	fake.variablesMutex.Unlock()
	if fake.VariablesStub != nil {
		return fake.VariablesStub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.variablesReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeKubeConverter) VariablesCallCount() int {
	fake.variablesMutex.RLock()
	defer fake.variablesMutex.RUnlock()
	return len(fake.variablesArgsForCall)
}

func (fake *FakeKubeConverter) VariablesCalls(stub func(string, []manifest.Variable) ([]v1alpha1.ExtendedSecret, error)) {
	fake.variablesMutex.Lock()
	defer fake.variablesMutex.Unlock()
	fake.VariablesStub = stub
}

func (fake *FakeKubeConverter) VariablesArgsForCall(i int) (string, []manifest.Variable) {
	fake.variablesMutex.RLock()
	defer fake.variablesMutex.RUnlock()
	argsForCall := fake.variablesArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeKubeConverter) VariablesReturns(result1 []v1alpha1.ExtendedSecret, result2 error) {
	fake.variablesMutex.Lock()
	defer fake.variablesMutex.Unlock()
	fake.VariablesStub = nil
	fake.variablesReturns = struct {
		result1 []v1alpha1.ExtendedSecret
		result2 error
	}{result1, result2}
}

func (fake *FakeKubeConverter) VariablesReturnsOnCall(i int, result1 []v1alpha1.ExtendedSecret, result2 error) {
	fake.variablesMutex.Lock()
	defer fake.variablesMutex.Unlock()
	fake.VariablesStub = nil
	if fake.variablesReturnsOnCall == nil {
		fake.variablesReturnsOnCall = make(map[int]struct {
			result1 []v1alpha1.ExtendedSecret
			result2 error
		})
	}
	fake.variablesReturnsOnCall[i] = struct {
		result1 []v1alpha1.ExtendedSecret
		result2 error
	}{result1, result2}
}

func (fake *FakeKubeConverter) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.bPMResourcesMutex.RLock()
	defer fake.bPMResourcesMutex.RUnlock()
	fake.variablesMutex.RLock()
	defer fake.variablesMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeKubeConverter) recordInvocation(key string, args []interface{}) {
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

var _ boshdeployment.KubeConverter = new(FakeKubeConverter)
