package mutate

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
)

// BoshDeploymentMutateFn returns MutateFn which mutates BoshDeployment including:
// - labels, annotations
// - spec
func BoshDeploymentMutateFn(boshDeployment *bdv1.BOSHDeployment) controllerutil.MutateFn {
	updated := boshDeployment.DeepCopy()
	return func() error {
		boshDeployment.Labels = updated.Labels
		boshDeployment.Annotations = updated.Annotations
		boshDeployment.Spec = updated.Spec
		return nil
	}
}

// EStsMutateFn returns MutateFn which mutates ExtendedStatefulSet including:
// - labels, annotations
// - spec
func EStsMutateFn(eSts *essv1.ExtendedStatefulSet) controllerutil.MutateFn {
	updated := eSts.DeepCopy()
	return func() error {
		eSts.Labels = updated.Labels
		eSts.Annotations = updated.Annotations
		eSts.Spec = updated.Spec
		return nil
	}
}

// EJobMutateFn returns MutateFn which mutates ExtendedJob including:
// - labels, annotations
// - spec.output, spec.Template, spec.trigger.podState, spec.updateOnConfigChange
func EJobMutateFn(eJob *ejv1.ExtendedJob) controllerutil.MutateFn {
	updated := eJob.DeepCopy()
	return func() error {
		eJob.Labels = updated.Labels
		eJob.Annotations = updated.Annotations
		// Does not reset Spec.Trigger.Strategy
		if len(eJob.Spec.Trigger.Strategy) == 0 {
			eJob.Spec.Trigger.Strategy = updated.Spec.Trigger.Strategy
		}
		eJob.Spec.Output = updated.Spec.Output
		eJob.Spec.Template = updated.Spec.Template
		eJob.Spec.UpdateOnConfigChange = updated.Spec.UpdateOnConfigChange
		return nil
	}
}

// ESecMutateFn returns MutateFn which mutates ExtendedSecret including:
// - labels, annotations
// - spec
// - status.generated
func ESecMutateFn(eSec *esv1.ExtendedSecret) controllerutil.MutateFn {
	updated := eSec.DeepCopy()
	return func() error {
		eSec.Labels = updated.Labels
		eSec.Annotations = updated.Annotations
		// Update only when spec has been changed
		if !reflect.DeepEqual(eSec.Spec, updated.Spec) {
			eSec.Spec = updated.Spec
			eSec.Status.Generated = updated.Status.Generated
		}

		return nil
	}
}

// SecretMutateFn returns MutateFn which mutates Secret including:
// - labels, annotations
// - stringData
func SecretMutateFn(s *corev1.Secret) controllerutil.MutateFn {
	updated := s.DeepCopy()
	return func() error {
		s.Labels = updated.Labels
		s.Annotations = updated.Annotations
		for key, data := range updated.StringData {
			// Update once one of data has been changed
			oriData, ok := s.Data[key]
			if ok && reflect.DeepEqual(string(oriData), data) {
				continue
			} else {
				s.StringData = updated.StringData
				break
			}
		}
		return nil
	}
}

// ServiceMutateFn returns MutateFn which mutates Service including:
// - labels, annotations
// - spec.ports, spec.selector
func ServiceMutateFn(svc *corev1.Service) controllerutil.MutateFn {
	updated := svc.DeepCopy()
	return func() error {
		svc.Labels = updated.Labels
		svc.Annotations = updated.Annotations
		// Should keep the existing ClusterIP
		svc.Spec.Ports = updated.Spec.Ports
		svc.Spec.Selector = updated.Spec.Selector
		return nil
	}
}
