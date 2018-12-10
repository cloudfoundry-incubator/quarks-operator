- [Makefile](#makefile)
- [CI](#ci)
- [Publishing](#publishing)
- [Creating a new Group and Controller](#creating-a-new-group-and-controller)
- [Versioning](#versioning)

## Makefile

Before starting, run `make tools` to install the required dependencies.

## CI

Our Concourse pipeline definitions are kept in the [https://github.com/cloudfoundry-incubator/cf-operator-ci](cf-operator-ci) repo.

## Publishing

## Creating a new Group and Controller

- create a new directory: `./pkg/kube/apis/<group_name>/<version>`
- in that directory, create the following files:
  - `types.go`
  - `register.go`
  - `doc.go`

  > You can safely use the implementation from another controller as inspiration.
  > You can also copy the files and modify them.

  The `types.go` file contains the definition of your resource. This is the file you care about. Make sure to run `make generate` _every time you make a change_. You can also check to see what changes would be done by running `make verify-gen-kube`.

  The `register.go` file contains some code that registers your new types.
  This file looks almost the same for all API groups.

  The `doc.go` (deep object copy) is required to make the `deepcopy` generator work.
  It's safe to copy this file from another controller.

- in `bin/gen-kube`, add your group to the `GROUP_VERSIONS` variable (separated by a space `" "`):

  ```bash
  # ...
  GROUP_VERSIONS="boshdeployment:v1alpha1 <controller_name>:<version>"
  # ...
  ```

- regenerate code

  ```bash
  # int the root of the project
  make generate
  ```

- create a directory structure like this for your actual controller code:

  ```
  .
  +-- pkg
     +-- kube
         +-- controllers
             +-- <controller_name>
             ¦   +-- controller.go
             ¦   +-- reconciler.go
             +-- controller.go
  ```

  - `controller.go` is your controller implementation; this is where you should implement an `Add` function where register the controller with the `Manager`, and you watch for changes for resources that you care about.
  - `reconciler.go` contains the code that takes action and reconciles actual state with desired state.

  Simple implementation to get you started below.
  As always, use the other implementations to get you started.

  **Controller:**

  ```go
  package mycontroller

  import (
    "go.uber.org/zap"

    "sigs.k8s.io/controller-runtime/pkg/controller"
    "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
    "sigs.k8s.io/controller-runtime/pkg/handler"
    "sigs.k8s.io/controller-runtime/pkg/manager"
    "sigs.k8s.io/controller-runtime/pkg/source"

    mrcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/myresourcecontroller/v1"
  )

  func Add(log *zap.SugaredLogger, mgr manager.Manager) error {
    r := NewReconciler(log, mgr, controllerutil.SetControllerReference)

    // Create a new controller
    c, err := controller.New("myresource-controller", mgr, controller.Options{Reconciler: r})
    if err != nil {
      return err
    }

    // Watch for changes to primary resource
    err = c.Watch(&source.Kind{Type: &mrcv1.MyResource{}}, &handler.EnqueueRequestForObject{})
    if err != nil {
      return err
    }

    return nil
  }
  ```

  **Reconciler:**
  ```go
  package myresource

  import (
    "go.uber.org/zap"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/manager"
    "sigs.k8s.io/controller-runtime/pkg/reconcile"
  )

  type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

  func NewReconciler(log *zap.SugaredLogger, mgr manager.Manager, srf setReferenceFunc) reconcile.Reconciler {
    return &ReconcileMyResource{
      log:          log,
      client:       mgr.GetClient(),
      scheme:       mgr.GetScheme(),
      setReference: srf,
    }
  }

  type ReconcileMyResource struct {
    client       client.Client
    scheme       *runtime.Scheme
    setReference setReferenceFunc
    log          *zap.SugaredLogger
  }

  func (r *ReconcileMyResource) Reconcile(request reconcile.Request) (reconcile.Result, error) {
    r.log.Infof("Reconciling MyResource %s\n", request.NamespacedName)
    return reconcile.Result{}, nil
  }
  ```

  Add the new group to `addToSchemes` in `pkg/controllers/controller.go`.
  Add the new controller to `addToManagerFuncs` in the same file.

## Versioning

APIs and types follow the upstream versioning scheme described at: https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-versioning
