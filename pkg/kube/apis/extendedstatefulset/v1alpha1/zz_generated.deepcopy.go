// +build !ignore_autogenerated

/*

Don't alter this file, it was generated.

*/
// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExtendedStatefulSet) DeepCopyInto(out *ExtendedStatefulSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExtendedStatefulSet.
func (in *ExtendedStatefulSet) DeepCopy() *ExtendedStatefulSet {
	if in == nil {
		return nil
	}
	out := new(ExtendedStatefulSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ExtendedStatefulSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExtendedStatefulSetList) DeepCopyInto(out *ExtendedStatefulSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ExtendedStatefulSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExtendedStatefulSetList.
func (in *ExtendedStatefulSetList) DeepCopy() *ExtendedStatefulSetList {
	if in == nil {
		return nil
	}
	out := new(ExtendedStatefulSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ExtendedStatefulSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExtendedStatefulSetSpec) DeepCopyInto(out *ExtendedStatefulSetSpec) {
	*out = *in
	if in.Zones != nil {
		in, out := &in.Zones, &out.Zones
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	in.Template.DeepCopyInto(&out.Template)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExtendedStatefulSetSpec.
func (in *ExtendedStatefulSetSpec) DeepCopy() *ExtendedStatefulSetSpec {
	if in == nil {
		return nil
	}
	out := new(ExtendedStatefulSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExtendedStatefulSetStatus) DeepCopyInto(out *ExtendedStatefulSetStatus) {
	*out = *in
	if in.LastReconcile != nil {
		in, out := &in.LastReconcile, &out.LastReconcile
		*out = (*in).DeepCopy()
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExtendedStatefulSetStatus.
func (in *ExtendedStatefulSetStatus) DeepCopy() *ExtendedStatefulSetStatus {
	if in == nil {
		return nil
	}
	out := new(ExtendedStatefulSetStatus)
	in.DeepCopyInto(out)
	return out
}
