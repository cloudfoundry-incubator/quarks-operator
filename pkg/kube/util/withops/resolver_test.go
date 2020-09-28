package withops_test

import (
	"context"
	"net/http"

	boshtpl "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/go-test/deep"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdc "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/withops"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("WithOps", func() {
	var (
		replaceOpsStr string
		removeOpsStr  string
		opaqueOpsStr  string
		urlOpsStr     string

		validManifestPath string
		validOpsPath      string
		invalidOpsPath    string

		resolver         *withops.Resolver
		ctx              context.Context
		client           client.Client
		interpolator     *fakes.FakeInterpolator
		remoteFileServer *ghttp.Server
		expectedManifest *bdm.Manifest
	)

	BeforeEach(func() {
		_, log := testhelper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)
		validManifestPath = "/valid-manifest.yml"
		validOpsPath = "/valid-ops.yml"
		invalidOpsPath = "/invalid-ops.yml"

		replaceOpsStr = `
- type: replace
  path: /instance_groups/name=component1?/instances
  value: 2
`
		removeOpsStr = `
- type: remove
  path: /instance_groups/name=component2?
`
		opaqueOpsStr = `---
- type: replace
  path: /instance_groups/name=component1?/instances
  value: 3
`

		urlOpsStr = `---
- type: replace
  path: /instance_groups/name=component1?/instances
  value: 4`

		client = fakeClient.NewFakeClient(
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "base-manifest",
					Namespace: "default",
				},
				Data: map[string]string{bdc.ManifestSpecName: `---
instance_groups:
  - name: component1
    instances: 1
  - name: component2
    instances: 2
`},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "default",
				},
				Data: map[string][]byte{bdc.ManifestSpecName: []byte(`---
instance_groups:
  - name: component3
    instances: 1
  - name: component4
    instances: 2
`)},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "opaque-manifest",
					Namespace: "default",
				},
				Data: map[string][]byte{bdc.ManifestSpecName: []byte(`---
instance_groups:
  - name: component3
    instances: 1
  - name: component4
    instances: 2
`)},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "manifest-with-vars",
					Namespace: "default",
				},
				Data: map[string]string{bdc.ManifestSpecName: `---
name: foo
instance_groups:
  - name: component1
    instances: 1
  - name: component2
    instances: 2
    properties:
      password: ((foo-pass.password))
variables:
  - name: foo-pass
    type: password
  - name: router_ca
    type: certificate
    options:
      is_ca: true
      common_name: ((system_domain))
`},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "manifest-with-dns",
					Namespace: "default",
				},
				Data: map[string]string{bdc.ManifestSpecName: `---
name: manifest-with-dns
addons:
- name: bosh-dns-aliases
  jobs:
  - name: bosh-dns-aliases
    release: bosh-dns-aliases
    properties:
      aliases:
      - domain: 'uaa.service.cf.internal'
        targets:
        - query: '_'
          instance_group: singleton-uaa
          deployment: cf
          network: default
          domain: bosh
instance_groups:
  - name: component1
    instances: 1
    jobs:
    - name: job1
      properties:
        url: https://uaa.service.cf.internal:8443/test/
variables:
  - name: router_ca
    type: certificate
    options:
      is_ca: true
      common_name: uaa.service.cf.internal
`},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "manifest-with-multiline-implicit-var",
					Namespace: "default",
				},
				Data: map[string]string{bdc.ManifestSpecName: `---
name: foo
instance_groups:
  - name: component1
    instances: 1
    properties:
      ca: ((implicit_ca))
`},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "manifest-with-embedded-implicit-var",
					Namespace: "default",
				},
				Data: map[string]string{bdc.ManifestSpecName: `---
name: foo
instance_groups:
  - name: component1
    instances: 1
    properties:
      host: 'foo.((system_domain))'
`},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "manifest-with-multi-key-implicit-var",
					Namespace: "default",
				},
				Data: map[string]string{bdc.ManifestSpecName: `---
name: foo
instance_groups:
  - name: component1
    instances: 1
    properties:
      ssl:
        ca: '((ssl/ca))'
        cert: '((ssl/cert))'
        key: '((ssl/key))'
`},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "var-system-domain",
					Namespace: "default",
				},
				Data: map[string][]byte{"value": []byte("example.com")},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "var-implicit-ca",
					Namespace: "default",
				},
				Data: map[string][]byte{"value": []byte("complicated\n'multiline'\nstring")},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "var-ssl",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"ca":   []byte("the-ca"),
					"cert": []byte("the-cert"),
					"key":  []byte("the-key"),
				},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "replace-ops",
					Namespace: "default",
				},
				Data: map[string]string{bdc.OpsSpecName: replaceOpsStr},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "remove-ops",
					Namespace: "default",
				},
				Data: map[string]string{bdc.OpsSpecName: removeOpsStr},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-ref",
					Namespace: "default",
				},
				Data: map[string]string{},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-yaml",
					Namespace: "default",
				},
				Data: map[string]string{bdc.ManifestSpecName: "!yaml"},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-ops",
					Namespace: "default",
				},
				Data: map[string]string{bdc.OpsSpecName: `
- type: invalid-ops
   path: /name
   value: new-deployment
`},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "missing-key",
					Namespace: "default",
				},
				Data: map[string]string{bdc.OpsSpecName: `
- type: replace
   path: /missing_key
   value: desired_value
`},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "opaque-ops",
					Namespace: "default",
				},
				Data: map[string][]byte{bdc.OpsSpecName: []byte(opaqueOpsStr)},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "manifest-with-resources",
					Namespace: "default",
				},
				Data: map[string]string{bdc.ManifestSpecName: `---
instance_groups:
  - name: componentWithResources
    instances: 1
    properties:
      ca: ((implicit_ca))
    jobs:
    - name: job1
      properties:
        quarks:
          bpm:
            processes:
            - name: xxx
              requests:
                memory: 128Mi
                cpu: 5m
`},
			},
		)

		remoteFileServer = ghttp.NewServer()
		remoteFileServer.AllowUnhandledRequests = true

		remoteFileServer.RouteToHandler("GET", validManifestPath, ghttp.RespondWith(http.StatusOK, `---
instance_groups:
  - name: component5
    instances: 1`))
		remoteFileServer.RouteToHandler("GET", validOpsPath, ghttp.RespondWith(http.StatusOK, urlOpsStr))
		remoteFileServer.RouteToHandler("GET", invalidOpsPath, ghttp.RespondWith(http.StatusOK, `---
- type: invalid-type
  path: /key
  value: values`))

		interpolator = &fakes.FakeInterpolator{}
		newInterpolatorFunc := func() withops.Interpolator {
			return interpolator
		}
		resolver = withops.NewResolver(client, newInterpolatorFunc)
	})

	Describe("Manifest", func() {
		It("works for valid CRs by using config map", func() {
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "base-manifest",
					},
				},
			}
			expectedManifest = &bdm.Manifest{
				InstanceGroups: []*bdm.InstanceGroup{
					{
						Name:      "component1",
						Instances: 1,
						Properties: bdm.InstanceGroupProperties{
							Properties: map[string]interface{}{},
						}},
					{
						Name:      "component2",
						Instances: 2,
						Properties: bdm.InstanceGroupProperties{
							Properties: map[string]interface{}{},
						},
					},
				},
				AddOnsApplied: true,
			}

			manifest, implicitVars, err := resolver.Manifest(ctx, deployment, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(len(manifest.InstanceGroups)).To(Equal(2))
			Expect(deep.Equal(manifest, expectedManifest)).To(HaveLen(0))
			Expect(len(implicitVars)).To(Equal(0))
		})

		It("works for valid CRs by using secret", func() {
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.SecretReference,
						Name: "opaque-manifest",
					},
				},
			}
			expectedManifest = &bdm.Manifest{
				InstanceGroups: []*bdm.InstanceGroup{
					{
						Name:      "component3",
						Instances: 1,
						Properties: bdm.InstanceGroupProperties{
							Properties: map[string]interface{}{},
						},
					},
					{
						Name:      "component4",
						Instances: 2,
						Properties: bdm.InstanceGroupProperties{
							Properties: map[string]interface{}{},
						},
					},
				},
				AddOnsApplied: true,
			}

			manifest, implicitVars, err := resolver.Manifest(ctx, deployment, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(len(manifest.InstanceGroups)).To(Equal(2))
			Expect(deep.Equal(manifest, expectedManifest)).To(HaveLen(0))
			Expect(len(implicitVars)).To(Equal(0))
		})

		It("works for valid CRs by using URL", func() {
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.URLReference,
						Name: remoteFileServer.URL() + validManifestPath,
					},
				},
			}
			expectedManifest = &bdm.Manifest{
				InstanceGroups: []*bdm.InstanceGroup{
					{
						Name:      "component5",
						Instances: 1,
						Properties: bdm.InstanceGroupProperties{
							Properties: map[string]interface{}{},
						},
					},
				},
				AddOnsApplied: true,
			}

			manifest, implicitVars, err := resolver.Manifest(ctx, deployment, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(len(manifest.InstanceGroups)).To(Equal(1))
			Expect(deep.Equal(manifest, expectedManifest)).To(HaveLen(0))
			Expect(len(implicitVars)).To(Equal(0))
		})

		It("works for valid CRs containing one ops", func() {
			interpolator.InterpolateReturns([]byte(`---
instance_groups:
  - name: component1
    instances: 2
  - name: component2
    instances: 2
`), nil)

			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "base-manifest",
					},
					Ops: []bdc.ResourceReference{
						{
							Type: bdc.ConfigMapReference,
							Name: "replace-ops",
						},
					},
				},
			}
			expectedManifest = &bdm.Manifest{
				InstanceGroups: []*bdm.InstanceGroup{
					{
						Name:      "component1",
						Instances: 2,
						Properties: bdm.InstanceGroupProperties{
							Properties: map[string]interface{}{},
						},
					},
					{
						Name:      "component2",
						Instances: 2,
						Properties: bdm.InstanceGroupProperties{
							Properties: map[string]interface{}{},
						},
					},
				},
				AddOnsApplied: true,
			}

			manifest, implicitVars, err := resolver.Manifest(ctx, deployment, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(len(manifest.InstanceGroups)).To(Equal(2))
			Expect(deep.Equal(manifest, expectedManifest)).To(HaveLen(0))

			Expect(interpolator.AddOpsCallCount()).To(Equal(1))
			opsBytes := interpolator.AddOpsArgsForCall(0)
			Expect(string(opsBytes)).To(Equal(replaceOpsStr))
			Expect(len(implicitVars)).To(Equal(0))
		})

		It("works for valid CRs containing multi ops", func() {
			interpolator.InterpolateReturns([]byte(`---
instance_groups:
  - name: component1
    instances: 4
`), nil)

			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "base-manifest",
					},
					Ops: []bdc.ResourceReference{
						{
							Type: bdc.ConfigMapReference,
							Name: "replace-ops",
						},
						{
							Type: bdc.SecretReference,
							Name: "opaque-ops",
						},
						{
							Type: bdc.URLReference,
							Name: remoteFileServer.URL() + validOpsPath,
						},
						{
							Type: bdc.ConfigMapReference,
							Name: "remove-ops",
						},
					},
				},
			}
			expectedManifest = &bdm.Manifest{
				InstanceGroups: []*bdm.InstanceGroup{
					{
						Name:      "component1",
						Instances: 4,
						Properties: bdm.InstanceGroupProperties{
							Properties: map[string]interface{}{},
						},
					},
				},
				AddOnsApplied: true,
			}

			manifest, implicitVars, err := resolver.Manifest(ctx, deployment, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(len(manifest.InstanceGroups)).To(Equal(1))
			Expect(deep.Equal(manifest, expectedManifest)).To(HaveLen(0))

			Expect(interpolator.AddOpsCallCount()).To(Equal(4))
			opsBytes := interpolator.AddOpsArgsForCall(0)
			Expect(string(opsBytes)).To(Equal(replaceOpsStr))
			opsBytes = interpolator.AddOpsArgsForCall(1)
			Expect(string(opsBytes)).To(Equal(opaqueOpsStr))
			opsBytes = interpolator.AddOpsArgsForCall(2)
			Expect(string(opsBytes)).To(Equal(urlOpsStr))
			opsBytes = interpolator.AddOpsArgsForCall(3)
			Expect(string(opsBytes)).To(Equal(removeOpsStr))
			Expect(len(implicitVars)).To(Equal(0))
		})

		It("works for resource requirements", func() {
			deployment := &bdc.BOSHDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo-deployment",
				},
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "manifest-with-resources",
					},
				},
			}

			manifest, _, err := resolver.Manifest(ctx, deployment, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(manifest).ToNot(Equal(nil))
			Expect(len(manifest.InstanceGroups)).To(Equal(1))
			resourceList := manifest.InstanceGroups[0].Jobs[0].Properties.Quarks.BPM.Processes[0].Requests
			Expect(resourceList.Memory().String()).To(Equal("128Mi"))
			Expect(resourceList.Cpu().String()).To(Equal("5m"))
		})

		It("throws an error if the manifest can not be found", func() {
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "not-existing",
					},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to retrieve manifest"))
		})

		It("throws an error if the CR is empty", func() {
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "empty-ref",
					},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("doesn't contain key 'manifest'"))
		})

		It("throws an error on invalid yaml", func() {
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "invalid-yaml",
					},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot unmarshal string into Go value of type manifest.Manifest"))
		})

		It("throws an error if containing unsupported manifest type", func() {
			interpolator.InterpolateReturns(nil, errors.New("fake-error"))
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Name: "base-manifest",
					},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unrecognized manifest ref type"))
		})

		It("throws an error if ops configMap can not be found", func() {
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "base-manifest",
					},
					Ops: []bdc.ResourceReference{
						{
							Type: bdc.ConfigMapReference,
							Name: "not-existing",
						},
					},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to retrieve ops from configmap"))
		})

		It("throws an error if ops configMap is empty", func() {
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "base-manifest",
					},
					Ops: []bdc.ResourceReference{
						{
							Type: bdc.ConfigMapReference,
							Name: "empty-ref",
						},
					},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("doesn't contain key 'ops'"))
		})

		It("throws an error if build invalid ops", func() {
			interpolator.AddOpsReturns(errors.New("fake-error"))

			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "base-manifest",
					},
					Ops: []bdc.ResourceReference{
						{
							Type: bdc.ConfigMapReference,
							Name: "invalid-ops",
						},
					},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Interpolation failed for bosh deployment"))
		})

		It("throws an error if interpolate a missing key into a manifest", func() {
			interpolator.InterpolateReturns(nil, errors.New("fake-error"))
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "base-manifest",
					},
					Ops: []bdc.ResourceReference{
						{
							Type: bdc.ConfigMapReference,
							Name: "missing-key",
						},
					},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to interpolate"))
		})

		It("throws an error if containing unsupported ops type", func() {
			interpolator.InterpolateReturns(nil, errors.New("fake-error"))
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "base-manifest",
					},
					Ops: []bdc.ResourceReference{
						{
							Name: "variables",
						},
					},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unrecognized ops ref type"))
		})

		It("throws an error if one config map can not be found when contains multi-ops", func() {
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "base-manifest",
					},
					Ops: []bdc.ResourceReference{
						{
							Type: bdc.SecretReference,
							Name: "opaque-ops",
						},
						{
							Type: bdc.ConfigMapReference,
							Name: "not-existing",
						},
					},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to retrieve ops from configmap"))
		})

		It("throws an error if one secret can not be found when contains multi-ops", func() {
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "base-manifest",
					},
					Ops: []bdc.ResourceReference{
						{
							Type: bdc.SecretReference,
							Name: "not-existing",
						},
						{
							Type: bdc.ConfigMapReference,
							Name: "replace-ops",
						},
					},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to retrieve ops from secret"))
		})

		It("throws an error if one url ref can not be found when contains multi-ops", func() {
			deployment := &bdc.BOSHDeployment{
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "base-manifest",
					},
					Ops: []bdc.ResourceReference{
						{
							Type: bdc.ConfigMapReference,
							Name: "replace-ops",
						},
						{
							Type: bdc.SecretReference,
							Name: "ops-secret",
						},
						{
							Type: bdc.URLReference,
							Name: remoteFileServer.URL() + "/not-found-ops.yml",
						},
					},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to retrieve ops from secret"))
		})

		It("replaces implicit variables", func() {
			deployment := &bdc.BOSHDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo-deployment",
				},
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "manifest-with-vars",
					},
					Ops: []bdc.ResourceReference{},
				},
			}
			m, implicitVars, err := resolver.Manifest(ctx, deployment, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(m.Variables[1].Options.CommonName).To(Equal("example.com"))
			Expect(len(implicitVars)).To(Equal(1))
			Expect(implicitVars[0]).To(Equal("var-system-domain"))
		})

		It("verify does not return an error for valid addon job properties", func() {
			deploymentName := "scf"
			newInterpolatorFunc := func() withops.Interpolator {
				return interpolator
			}
			resolver = withops.NewResolver(client, newInterpolatorFunc)

			deployment := &bdc.BOSHDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: deploymentName,
				},
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "manifest-with-dns",
					},
					Ops: []bdc.ResourceReference{},
				},
			}
			_, _, err := resolver.Manifest(ctx, deployment, "default")
			Expect(err).ToNot(HaveOccurred())
		})

		It("handles multi-line implicit vars", func() {
			deployment := &bdc.BOSHDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo-deployment",
				},
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "manifest-with-multiline-implicit-var",
					},
					Ops: []bdc.ResourceReference{},
				},
			}
			m, implicitVars, err := resolver.Manifest(ctx, deployment, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(len(implicitVars)).To(Equal(1))
			Expect(implicitVars[0]).To(Equal("var-implicit-ca"))
			Expect(m.InstanceGroups[0].Properties.Properties["ca"]).To(Equal("complicated\n'multiline'\nstring"))
		})

		It("handles embedded variables", func() {
			deployment := &bdc.BOSHDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo-deployment",
				},
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "manifest-with-embedded-implicit-var",
					},
					Ops: []bdc.ResourceReference{},
				},
			}
			m, implicitVars, err := resolver.Manifest(ctx, deployment, "default")

			Expect(err).ToNot(HaveOccurred())
			Expect(len(implicitVars)).To(Equal(1))
			Expect(implicitVars[0]).To(Equal("var-system-domain"))
			Expect(m.InstanceGroups[0].Properties.Properties["host"]).To(Equal("foo.example.com"))
		})

		It("handles multi-key implicit variables", func() {
			deployment := &bdc.BOSHDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo-deployment",
				},
				Spec: bdc.BOSHDeploymentSpec{
					Manifest: bdc.ResourceReference{
						Type: bdc.ConfigMapReference,
						Name: "manifest-with-multi-key-implicit-var",
					},
					Ops: []bdc.ResourceReference{},
				},
			}
			m, implicitVars, err := resolver.Manifest(ctx, deployment, "default")

			sslProps := m.InstanceGroups[0].Properties.Properties["ssl"].(map[string]interface{})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(implicitVars)).To(Equal(3))
			Expect(sslProps["ca"]).To(Equal("the-ca"))
			Expect(sslProps["cert"]).To(Equal("the-cert"))
			Expect(sslProps["key"]).To(Equal("the-key"))
		})
	})

	Context("Interpolate variables correctly", func() {
		var (
			baseManifest          []byte
			incorrectBaseManifest []byte
			vars                  []boshtpl.Variables
		)

		BeforeEach(func() {
			baseManifest = []byte(`
---
director_uuid: ((password1))
instance_groups:
- name: ((value1.key1))
- name: ((value2.key2))
- name: ((value2.key3))
`)
			incorrectBaseManifest = []byte(`
---
director_uuid: ((password2))
instance_groups:
- name: ((value1.key1))
- name: ((value2.key2))
- name: ((value2.key3))
`)
		})

		It("returns interpolated manifest", func() {
			vars = []boshtpl.Variables{
				0: boshtpl.StaticVariables{
					"password1": "password1data",
				},
				1: boshtpl.StaticVariables{
					"value1": map[interface{}]interface{}{
						"key1": "key1data",
					},
				},
				2: boshtpl.StaticVariables{
					"value2": map[interface{}]interface{}{
						"key2": "key2data",
						"key3": "ky3data",
					},
				},
			}
			desiredManifestBytes, err := withops.InterpolateExplicitVariables(baseManifest, vars, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(desiredManifestBytes)).To(Equal(`director_uuid: password1data
instance_groups:
- azs: null
  env:
    bosh:
      agent:
        settings: {}
      ipv6:
        enable: false
  instances: 0
  jobs: null
  name: key1data
  properties:
    quarks: {}
  stemcell: ""
  vm_resources: null
- azs: null
  env:
    bosh:
      agent:
        settings: {}
      ipv6:
        enable: false
  instances: 0
  jobs: null
  name: key2data
  properties:
    quarks: {}
  stemcell: ""
  vm_resources: null
- azs: null
  env:
    bosh:
      agent:
        settings: {}
      ipv6:
        enable: false
  instances: 0
  jobs: null
  name: ky3data
  properties:
    quarks: {}
  stemcell: ""
  vm_resources: null
`))
		})

		It("raises error when all variables in the manifest are not rendered", func() {
			vars = []boshtpl.Variables{
				0: boshtpl.StaticVariables{
					"password1": "password1data",
				},
				1: boshtpl.StaticVariables{
					"value1": map[interface{}]interface{}{
						"key1": "key1data",
					},
				},
				2: boshtpl.StaticVariables{
					"value2": map[interface{}]interface{}{
						"key2": "key2data",
						"key3": "ky3data",
					},
				},
			}
			_, err := withops.InterpolateExplicitVariables(incorrectBaseManifest, vars, true)
			Expect(err).To(HaveOccurred())
		})
	})
})
