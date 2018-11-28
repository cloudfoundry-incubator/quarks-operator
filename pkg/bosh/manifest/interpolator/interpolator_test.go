package interpolator_test

import (
	ipl "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/interpolator"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resolver", func() {
	var (
		baseManifest     []byte
		ops              []byte
		expectedManifest []byte
		interpolator     *ipl.InterpolatorImpl
	)

	BeforeEach(func() {
		interpolator = ipl.NewInterpolator()
	})

	Describe("BuildOps", func() {
		//Test for Hash
		It("works for setting a key", func() {
			ops = []byte(`
- type: replace
  path: /name
  value: new-deployment
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())
		})

		It("throws an error if deserialize invalid ops data", func() {
			ops = []byte(`
- type: replace
wrong-key
  path: /name
  value: new-deployment
`)

			err := interpolator.BuildOps(ops)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Deserializing ops data"))
		})

		It("throws an error if build invalid ops", func() {
			ops = []byte(`
- type: invalid-ops
  path: /name
  value: new-deployment
`)

			err := interpolator.BuildOps(ops)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Building ops"))
		})
	})

	Describe("Interpolate", func() {
		BeforeEach(func() {
			baseManifest = []byte(`
name: my-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.1
- 192.168.0.2
instance-groups:
  - name: diego
    instances: 3
  - name: mysql
    instances: 2
`)
		})

		//Test for Hash
		It("works for setting a key", func() {
			ops = []byte(`
- type: replace
  path: /name
  value: new-deployment
`)
			expectedManifest = []byte(`
name: new-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.1
- 192.168.0.2
instance-groups:
  - name: diego
    instances: 3
  - name: mysql
    instances: 2
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		It("works for removing a key", func() {
			ops = []byte(`
- type: remove
  path: /director_uuid
`)
			expectedManifest = []byte(`
name: my-deployment
dns:
- 192.168.0.1
- 192.168.0.2
instance-groups:
  - name: diego
    instances: 3
  - name: mysql
    instances: 2
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		It("works for adding a key", func() {
			ops = []byte(`
- type: replace
  path: /new_key?
  value: 1234abcd
`)
			expectedManifest = []byte(`
name: my-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.1
- 192.168.0.2
instance-groups:
  - name: diego
    instances: 3
  - name: mysql
    instances: 2
new_key: 1234abcd
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		It("throws an error if set a missing key", func() {
			ops = []byte(`
- type: replace
  path: /missing_key
  value: 1234abcd
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			_, err = interpolator.Interpolate(baseManifest)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Expected to find a map key 'missing_key'"))
		})

		//Teste for Array
		It("works for setting an item", func() {
			ops = []byte(`
- type: replace
  path: /dns/0
  value: 192.168.0.3
`)
			expectedManifest = []byte(`
name: my-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.3
- 192.168.0.2
instance-groups:
  - name: diego
    instances: 3
  - name: mysql
    instances: 2
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		It("works for removing an item", func() {
			ops = []byte(`
- type: remove
  path: /dns/1
`)
			expectedManifest = []byte(`
name: my-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.1
instance-groups:
  - name: diego
    instances: 3
  - name: mysql
    instances: 2
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		It("works for appending an item", func() {
			ops = []byte(`
- type: replace
  path: /dns/-
  value: 192.168.0.3
`)
			expectedManifest = []byte(`
name: my-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.1
- 192.168.0.2
- 192.168.0.3
instance-groups:
  - name: diego
    instances: 3
  - name: mysql
    instances: 2
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		It("works for replaceing 0th item before 1st", func() {
			ops = []byte(`
- type: replace
  path: /dns/1:prev
  value: 192.168.0.4
`)
			expectedManifest = []byte(`
name: my-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.4
- 192.168.0.2
instance-groups:
  - name: diego
    instances: 3
  - name: mysql
    instances: 2
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		It("works for inserting after 0th item", func() {
			ops = []byte(`
- type: replace
  path: /dns/0:after
  value: 192.168.0.3
`)
			expectedManifest = []byte(`
name: my-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.1
- 192.168.0.3
- 192.168.0.2
instance-groups:
  - name: diego
    instances: 3
  - name: mysql
    instances: 2
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		//Test for value
		It("works for modifying one existing variable", func() {
			ops = []byte(`
- type: replace
  path: /instance-groups/name=diego/instances
  value: 4
`)
			expectedManifest = []byte(`instance-groups:
name: my-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.1
- 192.168.0.2
instance-groups:
  - name: diego
    instances: 4
  - name: mysql
    instances: 2
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		It("works for modifying one existing variable with question mark", func() {
			ops = []byte(`
- type: replace
  path: /instance-groups/name=diego?/instances
  value: 2
`)
			expectedManifest = []byte(`instance-groups:
name: my-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.1
- 192.168.0.2
instance-groups:
  - name: diego
    instances: 2
  - name: mysql
    instances: 2
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		It("works for adding one root variable", func() {
			ops = []byte(`
- type: replace
  path: /instance-groups?/name=api/instances
  value: 2
`)
			expectedManifest = []byte(`instance-groups:
name: my-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.1
- 192.168.0.2
instance-groups:
  - name: diego
    instances: 3
  - name: mysql
    instances: 2
  - name: api
    instances: 2
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		It("works for adding one variable", func() {
			ops = []byte(`
- type: replace
  path: /instance-groups/name=mysql?/instances
  value: 1
`)
			expectedManifest = []byte(`instance-groups:
name: my-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.1
- 192.168.0.2
instance-groups:
  - name: diego
    instances: 3
  - name: mysql
    instances: 1
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		It("works for removing one variable", func() {
			ops = []byte(`
- type: remove
  path: /instance-groups/name=diego?
`)
			expectedManifest = []byte(`instance-groups:
name: my-deployment
director_uuid: 1234abcd
dns:
- 192.168.0.1
- 192.168.0.2
instance-groups:
  - name: mysql
    instances: 2
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			result, err := interpolator.Interpolate(baseManifest)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(MatchYAML(expectedManifest))
		})

		It("throws an error if modify one non-existing variable", func() {
			ops = []byte(`
- type: replace
  path: /instance-groups/name=missing-key/instances
  value: 2
`)

			err := interpolator.BuildOps(ops)
			Expect(err).ToNot(HaveOccurred())

			_, err = interpolator.Interpolate(baseManifest)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Expected to find exactly one matching array item for path"))
		})
	})
})
