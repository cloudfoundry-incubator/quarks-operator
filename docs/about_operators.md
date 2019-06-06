# About Operators

## Framework: Controller Runtime

* [Kubebuilder docs](https://book.kubebuilder.io/)
* [controller-runtime docs](https://godoc.org/github.com/kubernetes-sigs/controller-runtime/pkg)

## Operator Pattern & Features

* Operator pattern
  * [Kubernetes Custom Resource Controller](https://admiralty.io/blog/kubernetes-custom-resource-controller-and-operator-development-tools/)
  * [The Kubernetes Operator Pattern](https://www.slideshare.net/Jakobkaralus/the-kubernetes-operator-pattern-containerconf-nov-2017)

* Admission webhooks and eventing
  * [Sample Webhook](https://book.kubebuilder.io/beyond_basics/sample_webhook.html)
  * [Custom Resource Definitions](https://schd.ws/hosted_files/kccncchina2018english/50/kubecon_Tom_Ilya_CRDs.pdf)

* Finalizers
  * [Finalizers - Official Docs](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers)
  * [Using Finalizers](https://github.com/giantswarm/operatorkit/blob/master/docs/using_finalizers.md)

* Watches
  * [Controller Watches](https://book.kubebuilder.io/beyond_basics/controller_watches.html)

* Generate resources
  * [Code Generation for Custom Resources](https://blog.openshift.com/kubernetes-deep-dive-code-generation-customresources/)

* Apply CRD
  * [Extending Kubernetes APIs using CRDs](https://medium.com/velotio-perspectives/extending-kubernetes-apis-with-custom-resource-definitions-crds-139c99ed3477)

## Operator Examples

* [Elastic Search Operator](https://github.com/upmc-enterprises/elasticsearch-operator)
* [Postgres Operator](https://github.com/zalando-incubator/postgres-operator)
* [Tensorflow Operator](https://github.com/kubeflow/tf-operator)
* [NATS Operator](https://github.com/nats-io/nats-operator)
* [Knative](https://github.com/knative/serving/blob/059bf5f8c193148e54ddac37fba337c2cf6496db/cmd/controller/main.go#L144)
* [Sample controller](https://github.com/kubernetes/sample-controller)

## Extending Kubernetes

* [Controller pattern](https://engineering.bitnami.com/articles/a-deep-dive-into-kubernetes-controllers.html)
* [Custom controllers](https://medium.com/@trstringer/create-kubernetes-controllers-for-core-and-custom-resources-62fc35ad64a3)
* [CRD openAPI validation](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#validation)
* [Kubernetes primitives (ebook)](https://www.amazon.de/Kubernetes-Design-Patterns-Extensions-container-cluster-ebook/dp/B07HSZHRHZ)

## Testing

* [Kubernetes docs](https://github.com/thtanaka/kubernetes/blob/master/docs/devel/testing.md#integration-tests)
* [Kubernetes fakes](https://itnext.io/testing-kubernetes-go-applications-f1f87502b6ef)
* [Magic tricks of testing](https://speakerdeck.com/skmetz/magic-tricks-of-testing-railsconf)
