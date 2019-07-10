package manifest_test

import (
	"reflect"
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	t "code.cloudfoundry.org/cf-operator/testing"
	"code.cloudfoundry.org/cf-operator/testing/boshmanifest"
)

var (
	dupSpaces = regexp.MustCompile(`\s{2,}`)
)

func getStructTagForName(field string, opts interface{}) string {
	st, _ := reflect.TypeOf(opts).Elem().FieldByName(field)
	return dupSpaces.ReplaceAllString(string(st.Tag), " ")
}

var _ = Describe("Manifest", func() {
	Describe("Tags", func() {
		Describe("Manifest", func() {
			var manifest *Manifest

			BeforeEach(func() {
				manifest = &Manifest{}
			})

			Describe("Name", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Name", manifest)).To(Equal(
						`yaml:"name"`,
					))
				})
			})

			Describe("DirectorUUID", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("DirectorUUID", manifest)).To(Equal(
						`yaml:"director_uuid"`,
					))
				})
			})

			Describe("InstanceGroups", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("InstanceGroups", manifest)).To(Equal(
						`yaml:"instance_groups,omitempty"`,
					))
				})
			})

			Describe("Features", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Features", manifest)).To(Equal(
						`yaml:"features,omitempty"`,
					))
				})
			})

			Describe("Tags", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Tags", manifest)).To(Equal(
						`yaml:"tags,omitempty"`,
					))
				})
			})

			Describe("Releases", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Releases", manifest)).To(Equal(
						`yaml:"releases,omitempty"`,
					))
				})
			})

			Describe("Stemcells", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Stemcells", manifest)).To(Equal(
						`yaml:"stemcells,omitempty"`,
					))
				})
			})

			Describe("AddOns", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("AddOns", manifest)).To(Equal(
						`yaml:"addons,omitempty"`,
					))
				})
			})

			Describe("Properties", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Properties", manifest)).To(Equal(
						`yaml:"properties,omitempty"`,
					))
				})
			})

			Describe("Variables", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Variables", manifest)).To(Equal(
						`yaml:"variables,omitempty"`,
					))
				})
			})

			Describe("Update", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Update", manifest)).To(Equal(
						`yaml:"update,omitempty"`,
					))
				})
			})
		})

		Describe("AddOn", func() {
			var addOn *AddOn

			BeforeEach(func() {
				addOn = &AddOn{}
			})

			Describe("Name", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Name", addOn)).To(Equal(
						`yaml:"name"`,
					))
				})
			})

			Describe("Jobs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Jobs", addOn)).To(Equal(
						`yaml:"jobs"`,
					))
				})
			})

			Describe("Include", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Include", addOn)).To(Equal(
						`yaml:"include,omitempty"`,
					))
				})
			})

			Describe("Exclude", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Exclude", addOn)).To(Equal(
						`yaml:"exclude,omitempty"`,
					))
				})
			})
		})

		Describe("AddOnPlacementRules", func() {
			var addOnPlacementRule *AddOnPlacementRules

			BeforeEach(func() {
				addOnPlacementRule = &AddOnPlacementRules{}
			})

			Describe("Stemcell", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Stemcell", addOnPlacementRule)).To(Equal(
						`yaml:"stemcell,omitempty"`,
					))
				})
			})

			Describe("Deployments", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Deployments", addOnPlacementRule)).To(Equal(
						`yaml:"deployments,omitempty"`,
					))
				})
			})

			Describe("Jobs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Jobs", addOnPlacementRule)).To(Equal(
						`yaml:"release,omitempty"`,
					))
				})
			})

			Describe("InstanceGroup", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("InstanceGroup", addOnPlacementRule)).To(Equal(
						`yaml:"instance_groups,omitempty"`,
					))
				})
			})

			Describe("Networks", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Networks", addOnPlacementRule)).To(Equal(
						`yaml:"networks,omitempty"`,
					))
				})
			})

			Describe("Teams", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Teams", addOnPlacementRule)).To(Equal(
						`yaml:"teams,omitempty"`,
					))
				})
			})
		})

		Describe("AddOnPlacementJob", func() {
			var addOnPlacementJob *AddOnPlacementJob

			BeforeEach(func() {
				addOnPlacementJob = &AddOnPlacementJob{}
			})

			Describe("Name", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Name", addOnPlacementJob)).To(Equal(
						`yaml:"name"`,
					))
				})
			})

			Describe("Release", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Release", addOnPlacementJob)).To(Equal(
						`yaml:"release"`,
					))
				})
			})
		})

		Describe("AddOnStemcell", func() {
			var addOnStemcell *AddOnStemcell

			BeforeEach(func() {
				addOnStemcell = &AddOnStemcell{}
			})

			Describe("OS", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("OS", addOnStemcell)).To(Equal(
						`yaml:"os"`,
					))
				})
			})
		})

		Describe("AddOnJob", func() {
			var addOnJob *AddOnJob

			BeforeEach(func() {
				addOnJob = &AddOnJob{}
			})

			Describe("Name", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Name", addOnJob)).To(Equal(
						`yaml:"name"`,
					))
				})
			})

			Describe("Release", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Release", addOnJob)).To(Equal(
						`yaml:"release"`,
					))
				})
			})

			Describe("Properties", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Properties", addOnJob)).To(Equal(
						`yaml:"properties,omitempty"`,
					))
				})
			})
		})

		Describe("Release", func() {
			var release *Release

			BeforeEach(func() {
				release = &Release{}
			})

			Describe("Name", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Name", release)).To(Equal(
						`yaml:"name"`,
					))
				})
			})

			Describe("Version", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Version", release)).To(Equal(
						`yaml:"version"`,
					))
				})
			})

			Describe("URL", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("URL", release)).To(Equal(
						`yaml:"url,omitempty"`,
					))
				})
			})

			Describe("SHA1", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("SHA1", release)).To(Equal(
						`yaml:"sha1,omitempty"`,
					))
				})
			})

			Describe("Stemcell", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Stemcell", release)).To(Equal(
						`yaml:"stemcell,omitempty"`,
					))
				})
			})
		})

		Describe("ReleaseStemcell", func() {
			var releaseStemcell *ReleaseStemcell

			BeforeEach(func() {
				releaseStemcell = &ReleaseStemcell{}
			})

			Describe("OS", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("OS", releaseStemcell)).To(Equal(
						`yaml:"os"`,
					))
				})
			})

			Describe("Version", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Version", releaseStemcell)).To(Equal(
						`yaml:"version"`,
					))
				})
			})
		})

		Describe("Stemcell", func() {
			var stemcell *Stemcell

			BeforeEach(func() {
				stemcell = &Stemcell{}
			})

			Describe("Alias", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Alias", stemcell)).To(Equal(
						`yaml:"alias"`,
					))
				})
			})

			Describe("OS", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("OS", stemcell)).To(Equal(
						`yaml:"os,omitempty"`,
					))
				})
			})

			Describe("Version", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Version", stemcell)).To(Equal(
						`yaml:"version"`,
					))
				})
			})

			Describe("Name", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Name", stemcell)).To(Equal(
						`yaml:"name,omitempty"`,
					))
				})
			})
		})

		Describe("Variable", func() {
			var variable *Variable

			BeforeEach(func() {
				variable = &Variable{}
			})

			Describe("Name", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Name", variable)).To(Equal(
						`yaml:"name"`,
					))
				})
			})

			Describe("Type", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Type", variable)).To(Equal(
						`yaml:"type"`,
					))
				})
			})

			Describe("Options", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Options", variable)).To(Equal(
						`yaml:"options,omitempty"`,
					))
				})
			})
		})

		Describe("VariableOptions", func() {
			var variableOption *VariableOptions

			BeforeEach(func() {
				variableOption = &VariableOptions{}
			})

			Describe("CommonName", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("CommonName", variableOption)).To(Equal(
						`yaml:"common_name"`,
					))
				})
			})

			Describe("AlternativeNames", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("AlternativeNames", variableOption)).To(Equal(
						`yaml:"alternative_names,omitempty"`,
					))
				})
			})

			Describe("IsCA", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("IsCA", variableOption)).To(Equal(
						`yaml:"is_ca"`,
					))
				})
			})

			Describe("CA", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("CA", variableOption)).To(Equal(
						`yaml:"ca,omitempty"`,
					))
				})
			})

			Describe("ExtendedKeyUsage", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("ExtendedKeyUsage", variableOption)).To(Equal(
						`yaml:"extended_key_usage,omitempty"`,
					))
				})
			})
		})

		Describe("Feature", func() {
			var feature *Feature

			BeforeEach(func() {
				feature = &Feature{}
			})

			Describe("ConvergeVariables", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("ConvergeVariables", feature)).To(Equal(
						`yaml:"converge_variables"`,
					))
				})
			})

			Describe("RandomizeAzPlacement", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("RandomizeAzPlacement", feature)).To(Equal(
						`yaml:"randomize_az_placement,omitempty"`,
					))
				})
			})

			Describe("UseDNSAddresses", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("UseDNSAddresses", feature)).To(Equal(
						`yaml:"use_dns_addresses,omitempty"`,
					))
				})
			})

			Describe("UseTmpfsJobConfig", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("UseTmpfsJobConfig", feature)).To(Equal(
						`yaml:"use_tmpfs_job_config,omitempty"`,
					))
				})
			})
		})

		Describe("InstanceGroup", func() {
			var instanceGroup *InstanceGroup

			BeforeEach(func() {
				instanceGroup = &InstanceGroup{}
			})

			Describe("Name", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Name", instanceGroup)).To(Equal(
						`yaml:"name"`,
					))
				})
			})

			Describe("Instances", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Instances", instanceGroup)).To(Equal(
						`yaml:"instances"`,
					))
				})
			})

			Describe("AZs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("AZs", instanceGroup)).To(Equal(
						`yaml:"azs"`,
					))
				})
			})

			Describe("Jobs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Jobs", instanceGroup)).To(Equal(
						`yaml:"jobs"`,
					))
				})
			})

			Describe("VMType", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("VMType", instanceGroup)).To(Equal(
						`yaml:"vm_type,omitempty"`,
					))
				})
			})

			Describe("VMExtensions", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("VMExtensions", instanceGroup)).To(Equal(
						`yaml:"vm_extensions,omitempty"`,
					))
				})
			})

			Describe("VMResources", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("VMResources", instanceGroup)).To(Equal(
						`yaml:"vm_resources"`,
					))
				})
			})

			Describe("Stemcell", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Stemcell", instanceGroup)).To(Equal(
						`yaml:"stemcell"`,
					))
				})
			})

			Describe("PersistentDisk", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("PersistentDisk", instanceGroup)).To(Equal(
						`yaml:"persistent_disk,omitempty"`,
					))
				})
			})

			Describe("PersistentDiskType", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("PersistentDiskType", instanceGroup)).To(Equal(
						`yaml:"persistent_disk_type,omitempty"`,
					))
				})
			})

			Describe("Networks", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Networks", instanceGroup)).To(Equal(
						`yaml:"networks,omitempty"`,
					))
				})
			})

			Describe("Update", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Update", instanceGroup)).To(Equal(
						`yaml:"update,omitempty"`,
					))
				})
			})

			Describe("MigratedFrom", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("MigratedFrom", instanceGroup)).To(Equal(
						`yaml:"migrated_from,omitempty"`,
					))
				})
			})

			Describe("LifeCycle", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("LifeCycle", instanceGroup)).To(Equal(
						`yaml:"lifecycle,omitempty"`,
					))
				})
			})

			Describe("Properties", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Properties", instanceGroup)).To(Equal(
						`yaml:"properties,omitempty"`,
					))
				})
			})

			Describe("Env", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Env", instanceGroup)).To(Equal(
						`yaml:"env,omitempty"`,
					))
				})
			})
		})

		Describe("AgentEnv", func() {
			var agentEnv *AgentEnv

			BeforeEach(func() {
				agentEnv = &AgentEnv{}
			})

			Describe("PersistentDiskFS", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("PersistentDiskFS", agentEnv)).To(Equal(
						`yaml:"persistent_disk_fs,omitempty"`,
					))
				})
			})

			Describe("PersistentDiskMountOptions", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("PersistentDiskMountOptions", agentEnv)).To(Equal(
						`yaml:"persistent_disk_mount_options,omitempty"`,
					))
				})
			})

			Describe("AgentEnvBoshConfig", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("AgentEnvBoshConfig", agentEnv)).To(Equal(
						`yaml:"bosh,omitempty"`,
					))
				})
			})
		})

		Describe("AgentEnvBoshConfig", func() {
			var agentEnvBoshConfig *AgentEnvBoshConfig

			BeforeEach(func() {
				agentEnvBoshConfig = &AgentEnvBoshConfig{}
			})

			Describe("Password", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Password", agentEnvBoshConfig)).To(Equal(
						`yaml:"password,omitempty"`,
					))
				})
			})

			Describe("KeepRootPassword", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("KeepRootPassword", agentEnvBoshConfig)).To(Equal(
						`yaml:"keep_root_password,omitempty"`,
					))
				})
			})

			Describe("RemoveDevTools", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("RemoveDevTools", agentEnvBoshConfig)).To(Equal(
						`yaml:"remove_dev_tools,omitempty"`,
					))
				})
			})

			Describe("RemoveStaticLibraries", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("RemoveStaticLibraries", agentEnvBoshConfig)).To(Equal(
						`yaml:"remove_static_libraries,omitempty"`,
					))
				})
			})

			Describe("SwapSize", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("SwapSize", agentEnvBoshConfig)).To(Equal(
						`yaml:"swap_size,omitempty"`,
					))
				})
			})

			Describe("IPv6", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("IPv6", agentEnvBoshConfig)).To(Equal(
						`yaml:"ipv6,omitempty"`,
					))
				})
			})

			Describe("JobDir", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("JobDir", agentEnvBoshConfig)).To(Equal(
						`yaml:"job_dir,omitempty"`,
					))
				})
			})

			Describe("Agent", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Agent", agentEnvBoshConfig)).To(Equal(
						`yaml:"agent,omitempty"`,
					))
				})
			})
		})

		Describe("Agent", func() {
			var agent *Agent

			BeforeEach(func() {
				agent = &Agent{}
			})

			Describe("Settings", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Settings", agent)).To(Equal(
						`yaml:"settings,omitempty"`,
					))
				})
			})

			Describe("Tmpfs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Tmpfs", agent)).To(Equal(
						`yaml:"tmpfs,omitempty"`,
					))
				})
			})
		})

		Describe("AgentSettings", func() {
			var agentSettings *AgentSettings

			BeforeEach(func() {
				agentSettings = &AgentSettings{}
			})

			Describe("Settings", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Annotations", agentSettings)).To(Equal(
						`yaml:"annotations,omitempty"`,
					))
				})
			})

			Describe("Tmpfs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Labels", agentSettings)).To(Equal(
						`yaml:"labels,omitempty"`,
					))
				})
			})
		})

		Describe("JobDir", func() {
			var jobDir *JobDir

			BeforeEach(func() {
				jobDir = &JobDir{}
			})

			Describe("Tmpfs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Tmpfs", jobDir)).To(Equal(
						`yaml:"tmpfs,omitempty"`,
					))
				})
			})

			Describe("TmpfsSize", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("TmpfsSize", jobDir)).To(Equal(
						`yaml:"tmpfs_size,omitempty"`,
					))
				})
			})
		})

		Describe("IPv6", func() {
			var ipv6 *IPv6

			BeforeEach(func() {
				ipv6 = &IPv6{}
			})

			Describe("Enable", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Enable", ipv6)).To(Equal(
						`yaml:"enable"`,
					))
				})
			})
		})

		Describe("MigratedFrom", func() {
			var migratedFrom *MigratedFrom

			BeforeEach(func() {
				migratedFrom = &MigratedFrom{}
			})

			Describe("Name", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Name", migratedFrom)).To(Equal(
						`yaml:"name"`,
					))
				})
			})

			Describe("Az", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Az", migratedFrom)).To(Equal(
						`yaml:"az,omitempty"`,
					))
				})
			})
		})

		Describe("Update", func() {
			var update *Update

			BeforeEach(func() {
				update = &Update{}
			})

			Describe("Canaries", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Canaries", update)).To(Equal(
						`yaml:"canaries"`,
					))
				})
			})

			Describe("MaxInFlight", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("MaxInFlight", update)).To(Equal(
						`yaml:"max_in_flight"`,
					))
				})
			})

			Describe("CanaryWatchTime", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("CanaryWatchTime", update)).To(Equal(
						`yaml:"canary_watch_time"`,
					))
				})
			})

			Describe("UpdateWatchTime", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("UpdateWatchTime", update)).To(Equal(
						`yaml:"update_watch_time"`,
					))
				})
			})

			Describe("Serial", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Serial", update)).To(Equal(
						`yaml:"serial,omitempty"`,
					))
				})
			})

			Describe("VMStrategy", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("VMStrategy", update)).To(Equal(
						`yaml:"vm_strategy,omitempty"`,
					))
				})
			})
		})

		Describe("Network", func() {
			var network *Network

			BeforeEach(func() {
				network = &Network{}
			})

			Describe("Name", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Name", network)).To(Equal(
						`yaml:"name"`,
					))
				})
			})

			Describe("StaticIps", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("StaticIps", network)).To(Equal(
						`yaml:"static_ips,omitempty"`,
					))
				})
			})

			Describe("Default", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Default", network)).To(Equal(
						`yaml:"default,omitempty"`,
					))
				})
			})
		})

		Describe("VMResource", func() {
			var vmResource *VMResource

			BeforeEach(func() {
				vmResource = &VMResource{}
			})

			Describe("CPU", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("CPU", vmResource)).To(Equal(
						`yaml:"cpu"`,
					))
				})
			})

			Describe("RAM", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("RAM", vmResource)).To(Equal(
						`yaml:"ram"`,
					))
				})
			})

			Describe("EphemeralDiskSize", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("EphemeralDiskSize", vmResource)).To(Equal(
						`yaml:"ephemeral_disk_size"`,
					))
				})
			})
		})

		Describe("Job", func() {
			var job *Job

			BeforeEach(func() {
				job = &Job{}
			})

			Describe("Name", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Name", job)).To(Equal(
						`yaml:"name"`,
					))
				})
			})

			Describe("Release", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Release", job)).To(Equal(
						`yaml:"release"`,
					))
				})
			})

			Describe("Consumes", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Consumes", job)).To(Equal(
						`yaml:"consumes,omitempty"`,
					))
				})
			})

			Describe("Provides", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Provides", job)).To(Equal(
						`yaml:"provides,omitempty"`,
					))
				})
			})

			Describe("Properties", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Properties", job)).To(Equal(
						`yaml:"properties,omitempty"`,
					))
				})
			})
		})
	})

	Describe("Functions", func() {
		var (
			env t.Catalog
		)

		manifest := &Manifest{}

		Describe("LoadYAML", func() {
			It("populates fields fo default manifest", func() {
				manifest, err := LoadYAML([]byte(boshmanifest.Default))
				Expect(err).NotTo(HaveOccurred())
				Expect(manifest).ToNot(BeNil())

				Expect(manifest.Name).To(Equal("foo-deployment"))
				Expect(manifest.InstanceGroups).To(HaveLen(2))

				ig := manifest.InstanceGroups[0]
				Expect(ig.Name).To(Equal("redis-slave"))
				Expect(ig.Instances).To(Equal(2))
				Expect(ig.Properties).To(HaveLen(1))
				Expect(ig.Properties["foo"]).To(Equal(map[interface{}]interface{}{"app_domain": "((app_domain))"}))

				settings := ig.Env.AgentEnvBoshConfig.Agent.Settings
				Expect(settings.Labels).To(Equal(map[string]string{"custom-label": "foo"}))
				Expect(settings.Annotations).To(Equal(map[string]string{"custom-annotation": "bar"}))

				Expect(ig.Jobs).To(HaveLen(1))
				job := ig.Jobs[0]
				Expect(job.Name).To(Equal("redis-server"))
			})

			It("populates affinity fields", func() {
				manifest, err := LoadYAML([]byte(boshmanifest.BPMReleaseWithAffinity))
				Expect(err).NotTo(HaveOccurred())

				Expect(manifest.Name).To(Equal("bpm"))

				ig := manifest.InstanceGroups[0]
				Expect(ig.Name).To(Equal("bpm1"))
				Expect(ig.Instances).To(Equal(2))

				affinity := ig.Env.AgentEnvBoshConfig.Agent.Settings.Affinity
				Expect(affinity).ToNot(BeNil())
				Expect(affinity.NodeAffinity).ToNot(BeNil())
				Expect(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms).To(HaveLen(1))
				selectors := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0]
				Expect(selectors.MatchFields).To(BeNil())
				Expect(selectors.MatchExpressions).To(HaveLen(1))
				match := selectors.MatchExpressions[0]
				Expect(match.Key).To(Equal("beta.kubernetes.io/os"))
				Expect(match.Values).To(HaveLen(2))
			})

			It("provide and consume fields are populated", func() {
				manifest, err := LoadYAML([]byte(boshmanifest.WithProviderAndConsumer))
				Expect(err).NotTo(HaveOccurred())

				ig := manifest.InstanceGroups[1]
				job := ig.Jobs[0]

				Expect(job.Consumes["doppler"]).To(Equal(map[interface{}]interface{}{"from": "doppler"}))
				Expect(job.Properties.BOSHContainerization.Consumes).To(HaveLen(0))
			})

			It("bpm fields are populated", func() {
				manifest, err := LoadYAML([]byte(boshmanifest.WithOverriddenBPMInfo))
				Expect(err).NotTo(HaveOccurred())

				ig := manifest.InstanceGroups[0]
				job := ig.Jobs[0]

				Expect(job.Properties.Properties).To(HaveLen(1))
				props := job.Properties.Properties["foo"]
				Expect(props).To(HaveLen(1))
				Expect(props).To(Equal(map[interface{}]interface{}{"app_domain": "((app_domain))"}))

				bc := job.Properties.BOSHContainerization
				Expect(bc).ToNot(BeNil())
				Expect(bc.Ports).To(HaveLen(1))
				Expect(bc.PreRenderScripts).To(HaveLen(1))

				Expect(bc.BPM.Processes).To(HaveLen(1))
				proc := bc.BPM.Processes[0]

				Expect(proc.Name).To(Equal("redis"))
				Expect(proc.Executable).To(Equal("/another/command"))
				Expect(proc.EphemeralDisk).To(BeTrue())
				Expect(proc.PersistentDisk).To(BeTrue())

				Expect(proc.Hooks.PreStart).To(Equal("/var/vcap/jobs/pxc-mysql/bin/cleanup-socket"))
				Expect(proc.Env["PATH"]).To(Equal("/usr/bin:/bin:/var/vcap/packages/percona-xtrabackup/bin:/var/vcap/packages/pxc/bin:/var/vcap/packages/socat/bin"))
				Expect(proc.Limits.OpenFiles).To(Equal(100000))

				Expect(proc.AdditionalVolumes).To(HaveLen(2))
				vol := proc.AdditionalVolumes[0]
				Expect(vol.Path).To(Equal("/var/vcap/sys/run/pxc-mysql"))
				Expect(vol.Writable).To(BeTrue())
			})
		})

		Describe("GetReleaseImage", func() {
			BeforeEach(func() {
				*manifest = env.DefaultBOSHManifest()
			})

			It("reports an error if the instance group was not found", func() {
				_, err := manifest.GetReleaseImage("unknown-instancegroup", "redis-server")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found"))
			})

			It("reports an error if the stemcell was not found", func() {
				manifest.Stemcells = []*Stemcell{}
				_, err := manifest.GetReleaseImage("redis-slave", "redis-server")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("stemcell could not be resolved"))
			})

			It("reports an error if the job was not found", func() {
				_, err := manifest.GetReleaseImage("redis-slave", "unknown-job")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found"))
			})

			It("reports an error if the release was not found", func() {
				manifest.Releases = []*Release{}
				_, err := manifest.GetReleaseImage("redis-slave", "redis-server")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found"))
			})

			It("calculates the release image name", func() {
				releaseImage, err := manifest.GetReleaseImage("redis-slave", "redis-server")
				Expect(err).ToNot(HaveOccurred())
				Expect(releaseImage).To(Equal("hub.docker.com/cfcontainerization/redis:opensuse-42.3-28.g837c5b3-30.263-7.0.0_234.gcd7d1132-36.15.0"))
			})

			It("uses the release stemcell information if it is set", func() {
				releaseImage, err := manifest.GetReleaseImage("diego-cell", "cflinuxfs3-rootfs-setup")
				Expect(err).ToNot(HaveOccurred())
				Expect(releaseImage).To(Equal("hub.docker.com/cfcontainerization/cflinuxfs3:opensuse-15.0-28.g837c5b3-30.263-7.0.0_233.gde0accd0-0.62.0"))
			})
		})

		Describe("InstanceGroupByName", func() {
			BeforeEach(func() {
				*manifest = env.DefaultBOSHManifest()
			})

			It("returns an error if the instance group does not exist", func() {
				_, err := manifest.InstanceGroupByName("foo")
				Expect(err).To(HaveOccurred())
			})

			It("returns the instance group if it exists", func() {
				ig, err := manifest.InstanceGroupByName("redis-slave")
				Expect(err).ToNot(HaveOccurred())
				Expect(ig.Name).To(Equal("redis-slave"))
			})
		})
	})
})
