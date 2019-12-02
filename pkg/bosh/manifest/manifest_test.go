package manifest_test

import (
	"reflect"
	"regexp"

	"k8s.io/utils/pointer"

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
						`json:"name"`,
					))
				})
			})

			Describe("DirectorUUID", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("DirectorUUID", manifest)).To(Equal(
						`json:"director_uuid"`,
					))
				})
			})

			Describe("InstanceGroups", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("InstanceGroups", manifest)).To(Equal(
						`json:"instance_groups,omitempty"`,
					))
				})
			})

			Describe("Features", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Features", manifest)).To(Equal(
						`json:"features,omitempty"`,
					))
				})
			})

			Describe("Tags", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Tags", manifest)).To(Equal(
						`json:"tags,omitempty"`,
					))
				})
			})

			Describe("Releases", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Releases", manifest)).To(Equal(
						`json:"releases,omitempty"`,
					))
				})
			})

			Describe("Stemcells", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Stemcells", manifest)).To(Equal(
						`json:"stemcells,omitempty"`,
					))
				})
			})

			Describe("AddOns", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("AddOns", manifest)).To(Equal(
						`json:"addons,omitempty"`,
					))
				})
			})

			Describe("Properties", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Properties", manifest)).To(Equal(
						`json:"properties,omitempty"`,
					))
				})
			})

			Describe("Variables", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Variables", manifest)).To(Equal(
						`json:"variables,omitempty"`,
					))
				})
			})

			Describe("Update", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Update", manifest)).To(Equal(
						`json:"update,omitempty"`,
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
						`json:"name"`,
					))
				})
			})

			Describe("Jobs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Jobs", addOn)).To(Equal(
						`json:"jobs"`,
					))
				})
			})

			Describe("Include", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Include", addOn)).To(Equal(
						`json:"include,omitempty"`,
					))
				})
			})

			Describe("Exclude", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Exclude", addOn)).To(Equal(
						`json:"exclude,omitempty"`,
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
						`json:"stemcell,omitempty"`,
					))
				})
			})

			Describe("Deployments", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Deployments", addOnPlacementRule)).To(Equal(
						`json:"deployments,omitempty"`,
					))
				})
			})

			Describe("Jobs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Jobs", addOnPlacementRule)).To(Equal(
						`json:"release,omitempty"`,
					))
				})
			})

			Describe("InstanceGroup", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("InstanceGroup", addOnPlacementRule)).To(Equal(
						`json:"instance_groups,omitempty"`,
					))
				})
			})

			Describe("Networks", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Networks", addOnPlacementRule)).To(Equal(
						`json:"networks,omitempty"`,
					))
				})
			})

			Describe("Teams", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Teams", addOnPlacementRule)).To(Equal(
						`json:"teams,omitempty"`,
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
						`json:"name"`,
					))
				})
			})

			Describe("Release", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Release", addOnPlacementJob)).To(Equal(
						`json:"release"`,
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
						`json:"os"`,
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
						`json:"name"`,
					))
				})
			})

			Describe("Release", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Release", addOnJob)).To(Equal(
						`json:"release"`,
					))
				})
			})

			Describe("Properties", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Properties", addOnJob)).To(Equal(
						`json:"properties,omitempty"`,
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
						`json:"name"`,
					))
				})
			})

			Describe("Version", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Version", release)).To(Equal(
						`json:"version"`,
					))
				})
			})

			Describe("URL", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("URL", release)).To(Equal(
						`json:"url,omitempty"`,
					))
				})
			})

			Describe("SHA1", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("SHA1", release)).To(Equal(
						`json:"sha1,omitempty"`,
					))
				})
			})

			Describe("Stemcell", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Stemcell", release)).To(Equal(
						`json:"stemcell,omitempty"`,
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
						`json:"os"`,
					))
				})
			})

			Describe("Version", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Version", releaseStemcell)).To(Equal(
						`json:"version"`,
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
						`json:"alias"`,
					))
				})
			})

			Describe("OS", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("OS", stemcell)).To(Equal(
						`json:"os,omitempty"`,
					))
				})
			})

			Describe("Version", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Version", stemcell)).To(Equal(
						`json:"version"`,
					))
				})
			})

			Describe("Name", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Name", stemcell)).To(Equal(
						`json:"name,omitempty"`,
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
						`json:"name"`,
					))
				})
			})

			Describe("Type", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Type", variable)).To(Equal(
						`json:"type"`,
					))
				})
			})

			Describe("Options", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Options", variable)).To(Equal(
						`json:"options,omitempty"`,
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
						`json:"common_name"`,
					))
				})
			})

			Describe("AlternativeNames", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("AlternativeNames", variableOption)).To(Equal(
						`json:"alternative_names,omitempty"`,
					))
				})
			})

			Describe("IsCA", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("IsCA", variableOption)).To(Equal(
						`json:"is_ca"`,
					))
				})
			})

			Describe("CA", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("CA", variableOption)).To(Equal(
						`json:"ca,omitempty"`,
					))
				})
			})

			Describe("ExtendedKeyUsage", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("ExtendedKeyUsage", variableOption)).To(Equal(
						`json:"extended_key_usage,omitempty"`,
					))
				})
			})

			Describe("SignerType", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("SignerType", variableOption)).To(Equal(
						`json:"signer_type,omitempty"`,
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
						`json:"converge_variables"`,
					))
				})
			})

			Describe("RandomizeAzPlacement", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("RandomizeAzPlacement", feature)).To(Equal(
						`json:"randomize_az_placement,omitempty"`,
					))
				})
			})

			Describe("UseDNSAddresses", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("UseDNSAddresses", feature)).To(Equal(
						`json:"use_dns_addresses,omitempty"`,
					))
				})
			})

			Describe("UseTmpfsJobConfig", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("UseTmpfsJobConfig", feature)).To(Equal(
						`json:"use_tmpfs_job_config,omitempty"`,
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
						`json:"name"`,
					))
				})
			})

			Describe("Instances", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Instances", instanceGroup)).To(Equal(
						`json:"instances"`,
					))
				})
			})

			Describe("AZs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("AZs", instanceGroup)).To(Equal(
						`json:"azs"`,
					))
				})
			})

			Describe("Jobs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Jobs", instanceGroup)).To(Equal(
						`json:"jobs"`,
					))
				})
			})

			Describe("VMType", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("VMType", instanceGroup)).To(Equal(
						`json:"vm_type,omitempty"`,
					))
				})
			})

			Describe("VMExtensions", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("VMExtensions", instanceGroup)).To(Equal(
						`json:"vm_extensions,omitempty"`,
					))
				})
			})

			Describe("VMResources", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("VMResources", instanceGroup)).To(Equal(
						`json:"vm_resources"`,
					))
				})
			})

			Describe("Stemcell", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Stemcell", instanceGroup)).To(Equal(
						`json:"stemcell"`,
					))
				})
			})

			Describe("PersistentDisk", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("PersistentDisk", instanceGroup)).To(Equal(
						`json:"persistent_disk,omitempty"`,
					))
				})
			})

			Describe("PersistentDiskType", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("PersistentDiskType", instanceGroup)).To(Equal(
						`json:"persistent_disk_type,omitempty"`,
					))
				})
			})

			Describe("Networks", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Networks", instanceGroup)).To(Equal(
						`json:"networks,omitempty"`,
					))
				})
			})

			Describe("Update", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Update", instanceGroup)).To(Equal(
						`json:"update,omitempty"`,
					))
				})
			})

			Describe("MigratedFrom", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("MigratedFrom", instanceGroup)).To(Equal(
						`json:"migrated_from,omitempty"`,
					))
				})
			})

			Describe("LifeCycle", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("LifeCycle", instanceGroup)).To(Equal(
						`json:"lifecycle,omitempty"`,
					))
				})
			})

			Describe("Properties", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Properties", instanceGroup)).To(Equal(
						`json:"properties,omitempty"`,
					))
				})
			})

			Describe("Env", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Env", instanceGroup)).To(Equal(
						`json:"env,omitempty"`,
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
						`json:"persistent_disk_fs,omitempty"`,
					))
				})
			})

			Describe("PersistentDiskMountOptions", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("PersistentDiskMountOptions", agentEnv)).To(Equal(
						`json:"persistent_disk_mount_options,omitempty"`,
					))
				})
			})

			Describe("AgentEnvBoshConfig", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("AgentEnvBoshConfig", agentEnv)).To(Equal(
						`json:"bosh,omitempty"`,
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
						`json:"password,omitempty"`,
					))
				})
			})

			Describe("KeepRootPassword", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("KeepRootPassword", agentEnvBoshConfig)).To(Equal(
						`json:"keep_root_password,omitempty"`,
					))
				})
			})

			Describe("RemoveDevTools", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("RemoveDevTools", agentEnvBoshConfig)).To(Equal(
						`json:"remove_dev_tools,omitempty"`,
					))
				})
			})

			Describe("RemoveStaticLibraries", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("RemoveStaticLibraries", agentEnvBoshConfig)).To(Equal(
						`json:"remove_static_libraries,omitempty"`,
					))
				})
			})

			Describe("SwapSize", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("SwapSize", agentEnvBoshConfig)).To(Equal(
						`json:"swap_size,omitempty"`,
					))
				})
			})

			Describe("IPv6", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("IPv6", agentEnvBoshConfig)).To(Equal(
						`json:"ipv6,omitempty"`,
					))
				})
			})

			Describe("JobDir", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("JobDir", agentEnvBoshConfig)).To(Equal(
						`json:"job_dir,omitempty"`,
					))
				})
			})

			Describe("Agent", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Agent", agentEnvBoshConfig)).To(Equal(
						`json:"agent,omitempty"`,
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
						`json:"settings,omitempty"`,
					))
				})
			})

			Describe("Tmpfs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Tmpfs", agent)).To(Equal(
						`json:"tmpfs,omitempty"`,
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
						`json:"annotations,omitempty"`,
					))
				})
			})

			Describe("Tmpfs", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Labels", agentSettings)).To(Equal(
						`json:"labels,omitempty"`,
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
						`json:"tmpfs,omitempty"`,
					))
				})
			})

			Describe("TmpfsSize", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("TmpfsSize", jobDir)).To(Equal(
						`json:"tmpfs_size,omitempty"`,
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
						`json:"enable"`,
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
						`json:"name"`,
					))
				})
			})

			Describe("Az", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Az", migratedFrom)).To(Equal(
						`json:"az,omitempty"`,
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
						`json:"canaries"`,
					))
				})
			})

			Describe("MaxInFlight", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("MaxInFlight", update)).To(Equal(
						`json:"max_in_flight"`,
					))
				})
			})

			Describe("CanaryWatchTime", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("CanaryWatchTime", update)).To(Equal(
						`json:"canary_watch_time"`,
					))
				})
			})

			Describe("UpdateWatchTime", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("UpdateWatchTime", update)).To(Equal(
						`json:"update_watch_time"`,
					))
				})
			})

			Describe("Serial", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Serial", update)).To(Equal(
						`json:"serial,omitempty"`,
					))
				})
			})

			Describe("VMStrategy", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("VMStrategy", update)).To(Equal(
						`json:"vm_strategy,omitempty"`,
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
						`json:"name"`,
					))
				})
			})

			Describe("StaticIps", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("StaticIps", network)).To(Equal(
						`json:"static_ips,omitempty"`,
					))
				})
			})

			Describe("Default", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Default", network)).To(Equal(
						`json:"default,omitempty"`,
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
						`json:"cpu"`,
					))
				})
			})

			Describe("RAM", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("RAM", vmResource)).To(Equal(
						`json:"ram"`,
					))
				})
			})

			Describe("EphemeralDiskSize", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("EphemeralDiskSize", vmResource)).To(Equal(
						`json:"ephemeral_disk_size"`,
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
						`json:"name"`,
					))
				})
			})

			Describe("Release", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Release", job)).To(Equal(
						`json:"release"`,
					))
				})
			})

			Describe("Consumes", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Consumes", job)).To(Equal(
						`json:"consumes,omitempty"`,
					))
				})
			})

			Describe("Provides", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Provides", job)).To(Equal(
						`json:"provides,omitempty"`,
					))
				})
			})

			Describe("Properties", func() {
				It("contains desired values", func() {
					Expect(getStructTagForName("Properties", job)).To(Equal(
						`json:"properties,omitempty"`,
					))
				})
			})
		})
	})

	Describe("Functions", func() {
		var (
			env t.Catalog
			err error
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
				Expect(ig.Properties.Properties).To(HaveLen(1))
				Expect(ig.Properties.Properties["foo"]).To(Equal(map[string]interface{}{"app_domain": "((app_domain))"}))

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

				Expect(manifest.Name).To(Equal("bpm-affinity"))

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

				Expect(job.Consumes["doppler"]).To(Equal(map[string]interface{}{"from": "doppler"}))
				Expect(job.Properties.Quarks.Consumes).To(HaveLen(0))
			})

			It("bpm fields are populated", func() {
				manifest, err := LoadYAML([]byte(boshmanifest.WithOverriddenBPMInfo))
				Expect(err).NotTo(HaveOccurred())

				ig := manifest.InstanceGroups[0]
				job := ig.Jobs[0]

				Expect(job.Properties.Properties).To(HaveLen(1))
				props := job.Properties.Properties["foo"]
				Expect(props).To(HaveLen(1))
				Expect(props).To(Equal(map[string]interface{}{"app_domain": "((app_domain))"}))

				bc := job.Properties.Quarks
				Expect(bc).ToNot(BeNil())
				Expect(bc.Ports).To(HaveLen(1))
				Expect(bc.PreRenderScripts.Jobs).To(HaveLen(1))
				Expect(bc.PreRenderScripts.BPM).To(BeNil())

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

		Describe("Marshal", func() {
			orgText := boshmanifest.BPMReleaseWithAffinity
			largeText := boshmanifest.ManifestWithLargeValues
			var m1 *Manifest
			var largeManifest *Manifest

			Context("with a manifest with large values", func() {
				BeforeEach(func() {
					var err error
					largeManifest, err = LoadYAML([]byte(largeText))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should produce the same result when run multiple times", func() {
					result1, err := largeManifest.Marshal()
					Expect(err).NotTo(HaveOccurred())
					result2, err := largeManifest.Marshal()
					Expect(err).NotTo(HaveOccurred())
					Expect(result1).To(Equal(result2))
				})

				It("should marshal correctly and resolve anchors", func() {
					marshalledLargeManifest, err := largeManifest.Marshal()
					Expect(err).NotTo(HaveOccurred())

					uncompressed_size := len([]byte(largeText))
					compressed_size := len([]byte(marshalledLargeManifest))
					Expect(compressed_size < uncompressed_size).To(BeTrue())

					By("Unmarshalling the large manifest")

					largeManifest, err = LoadYAML(marshalledLargeManifest)
					Expect(err).NotTo(HaveOccurred())

					Expect(largeManifest.InstanceGroups[0].Jobs[1].Properties.Properties["loggregator"].(map[string]interface{})["tls"].(map[string]interface{})["agent"].(map[string]interface{})["cert"]).To(Equal("-----BEGIN CERTIFICATE-----\nMIIFOzC" +
						"CAyOgAwIBAgIUUkyFTjBNZJyp4xFMwq9vImhV/OUwDQYJKoZIhvcNAQEN\nBQAwGDEWMBQGA1UEAxMNbG9nZ3JlZ2F0b3JDQTAeFw0xOTA3MjUwNjA2MDBaFw0y\nMDA3MjQwNjA2MDBaMBExDzANBgNVBAMTBm1ldHJvbjCCAiIwDQYJKoZIhvcNAQEB\nBQADggIPADCCAgoCggIBANRWsOAZNcCZghQsXr" +
						"QKaHOpdgibljN5K0ZeCXwsKbOa\nXoM8aNB5I+XxHFLYkB6zm5cXv8n6UHeiFaemxjSMT7shO/yTyYq6MpfSdHM1Eops\nLOrKCqXDwi+hxvQmTKxtmVb/Ja6RqnsVDaIkLL/DN803De8yEwPexxYWHMIKwSaY\nWaVYgZugp89HGzcoeX+N2WXmPOrqMi2OZ1ZC0+lUpUjC0EJYBn+oYF234VQSsCIi\nh++" +
						"AAFbgnzBV4xl8/NeGP1Xqqu57qlz3tFyFoj+k8iFa6Buz5Dv1+JAt+8MERplY\nnIDlHEfmD5TI9cPVDHnBp7Gth+Fv4s5RcnFLOUR+xWvIJ9XiqJUXtFaN0sTIC/DV\nIocg92NQDOLsCRNJV47jV4c1biMvV0AICZdlMebRRJRAgfd3Um4CriOnvYNsoFuC\nee10BeyiP1FPJz6dUeTXRgDq9aYlZf59Q6" +
						"3b0zaT1IYK0eHmTzlKduLn04dL5p/T\nvJIR6nSaHKdi6/XTKDnT3KuuDb/rYPPTHGprFW0czt/w0u3CSJFnoH5r9kbVZn7j\n4xMZoY3JPz8nzPU9tW6pNenc/vMWp5DYe2IlyiwkbUM5xAPKO9DxSxnn/aussuyB\nKJErotN20YGOZcGVskc5DwqrntWZFL1pFQf1IgcBzCjM6TomDHkp5Jn6Lqvad9xH\n" +
						"AgMBAAGjgYMwgYAwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1Ud\nEwEB/wQCMAAwHQYDVR0OBBYEFPHm52ztGFbCDLj65PEj81S058jNMB8GA1UdIwQY\nMBaAFBAuxNhA1yte0Ftw+0MRngdKVLnYMBEGA1UdEQQKMAiCBm1ldHJvbjANBgkq\nhkiG9w0BAQ0FAAOCAgEAFIVP3POc1jf" +
						"9mhTD2o9A2u+pm+LL8VBjPeA7X0PkFT3R\nVwG5CbAQqmY9giNBCV00RruYNE1qrlsM06kQnHbglqAIlEMFz50M9yzyYvKxw4uQ\nFSnSdEdl1rgF0G82Q2IA0jFCxZ8sz/GzGROBHbNv5FQs7leNYmykvUKkLJdwBskn\nCsZ7PA1V9mKMogD3BbqH3lB7nRwRmA1LMOSu50l6PJAH+gdTnVzV2QF6B9shJ+" +
						"dT\nTSzsL2GSjoAv0/F1jAVUbmroNyoZ7/KoAecRRedzGnpWDrRUsvktlGOhGpjd9f3S\nQWIn0KjvOiJVUygXBbvgJ8X5bGTyUgxKa02N4OaMHT18hPVjyhD5nzgq/hGrbjvf\ntFSEwgKan2080XjOeVubFhxcMVTp3gD6Q0EAsTuxaw1SYkbqXxb6rRBeIWkMavN/\ncRsgaLj16uNKXxHHRRQm0BV029u" +
						"dogqOQVqDwOlMDFFFSQmMgx1kWzcU4leyiaZT\nfrmOKKy0K6czUQ/tE4Bt9/7SLPIysMCDSxE4sPefS+m030LpaVgGidiEmc/Fs9pW\n/15rKzOePCVXG7IBzkNJmb0SRdCrG8sPn56O5Gc5EiULZJL24FJzRysToxf7RhFz\n2tZ5jxFlhSjRZLTxXAJirEcjAgzrpX+47D/UuWcQiuNdbSZk4MZuCFEbY" +
						"Vho9C8=\n-----END CERTIFICATE-----\n"))
					Expect(largeManifest.InstanceGroups[0].Jobs[3].Properties.Properties["tls"].(map[string]interface{})["ca_cert"]).To(Equal("-----BEGIN CERTIFICATE-----\nMIIFADCCAuigAwIBAgIUPVdppFi6U3l893jVxiW0gr760jUwDQYJKoZIhvcNAQEN\nBQAwGDEWMBQGA1" +
						"UEAxMNbG9nZ3JlZ2F0b3JDQTAeFw0xOTA3MjUwNjA2MDBaFw0y\nMDA3MjQwNjA2MDBaMBgxFjAUBgNVBAMTDWxvZ2dyZWdhdG9yQ0EwggIiMA0GCSqG\nSIb3DQEBAQUAA4ICDwAwggIKAoICAQCU8UVt42KUxm38od42zhsV3O/8g3eBmUem\n7IRER844NRHlci+nnVvemFdA81bbbDsgocljVhbFnGB1ELb" +
						"hNyEnqGrsk88Qou1s\nR/3wiSwg59TmLre4Kk2JbmRqzHcYJW22A4wUGspdjhchFMmstRryBCEV84IPHNH0\naZ2SJQHsciB0mag/avvPbQ9F76uJC/eA5mG0KqH23QC1nARCmcfKrmkeXD8qFmki\njH0nStrFVAlRX7SjNAd2N+64uVzisGO0lze+V8o7MAr7pJxzmPfGs0QYhFpFHgcO\nrOEvNW1HTanc8a" +
						"n338DDlZSSqdVqdBhRXXFSP75+D0y8UNajVxXzUvOJ3rZfNbFV\nLlnOTHW/ItiOJjzodUfhE3jzjv4DqvKIk/Mrp0HVpgH5niGWgF4LIAav7cK7fVgd\nxACtuUAhAsL3RFddvz8sY4ixm8O0jvAUerCRPnjnA+Uj/1i7XX9cjmIVfcxwjcfH\nmLFSnXtX6+w4m4tWEIN/BptwLdfnMB2DzRXbDQE7m+vxITf" +
						"BLaY/vK5NA8lil/n8\nFISPtLczIORvjkRrwPKLv435EUxd0EIJFVj7wKaWZDPmtIwOHex1n12BTzlfToig\nFrJi/KwwF4+GwnfERkJkd6JafB7/28Gqp6+UzXcKphBOjGDhaAu7/NlOteRsRLHs\nM0DxqcMh3QIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB\n/zAdBgNVHQ4EFg" +
						"QUEC7E2EDXK17QW3D7QxGeB0pUudgwDQYJKoZIhvcNAQENBQAD\nggIBAGUcEEk1dKdn73IapvFhrDKHNYSLEGgIVpyvnwjwi4EyXzHNhmGMnHJkAYRg\nKaWBfao8ngYawfEtFpvz1pdpOW+Ul8bMtcC+mJlxI/E/Od0WWNE6QRNdWsoH5JSj\nef+SepxE6ztMfzayC4Tmp85vT1TWi7/2maHuefosAKiwovt" +
						"csnr54Y6GJkozY2Hd\n46V185MuDK14BeS9Yne9XWSDOdjZH20kRHtoRbxRz15krFmbbpIyek2mss2nVV2d\nt1pUK4er6R4y3QHBn7QBq5kAxiKhFY6yA88+uhX2jf4u5uroG0CHGdZmKlGrb4N/\nfC/1BSBo16V6EOZAy35ktlg4oSCbeJmDXYwZzVvOpQGPRqB7lfDM1bZcv8vdxrXn\nYALcq7OVkRFeCy" +
						"9HDEvwARfQ1axTZM+tKrcQav7dIKNGr4inzg9tNBhtORlZudhi\nAfpHyEr6rMFk8t63Q45MXMp5L9x4ThyPjyfo17BwhfjY47ibbHvo4vy9O/vbcw4i\nNASFM8VUwtFO9Ip3GAVtUZR4V+i77SsDo3B8546T/KDP2cBjnP+sSjUvtpAGLDFJ\nHa4RWJN4IE+DdVIcipKT2yCzI3Xr8NUO+Q+h7wVgtE8e2sN" +
						"rsM5X76ILtZBlOfPy\njVdYnn9gIxqS6iWHiGfAHf4Bs+shXicXye88TfeNDnHvLw/Q\n-----END CERTIFICATE-----\n"))
				})
			})

			Context("with a regular manifest", func() {
				BeforeEach(func() {
					var err error
					m1, err = LoadYAML([]byte(orgText))
					Expect(err).NotTo(HaveOccurred())
				})

				It("converts k8s tags to lowercase", func() {
					text, err := m1.Marshal()
					Expect(err).NotTo(HaveOccurred())

					Expect(orgText).To(ContainSubstring("requiredDuringSchedulingIgnoredDuringExecution"))
					Expect(text).To(ContainSubstring("requiredDuringSchedulingIgnoredDuringExecution"))
				})

				It("retains affinity data", func() {
					text, err := m1.Marshal()
					Expect(err).NotTo(HaveOccurred())

					By("loading marshalled manifest again")
					manifest, err := LoadYAML(text)
					Expect(err).NotTo(HaveOccurred())

					ig := manifest.InstanceGroups[0]
					Expect(ig.Name).To(Equal("bpm1"))
					Expect(ig.Instances).To(Equal(2))

					affinity := ig.Env.AgentEnvBoshConfig.Agent.Settings.Affinity
					Expect(affinity).ToNot(BeNil())
					Expect(affinity.NodeAffinity).ToNot(BeNil())
					Expect(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms).To(HaveLen(1))
					selectors := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0]
					Expect(selectors.MatchFields).To(HaveLen(0))

					Expect(selectors.MatchExpressions).To(HaveLen(1))
					match := selectors.MatchExpressions[0]
					Expect(match.Key).To(Equal("beta.kubernetes.io/os"))
					Expect(match.Values).To(HaveLen(2))
				})

				It("retains healthcheck data", func() {
					defaultText := boshmanifest.Default
					m1, err := LoadYAML([]byte(defaultText))
					Expect(err).NotTo(HaveOccurred())
					text, err := m1.Marshal()
					Expect(err).NotTo(HaveOccurred())

					By("loading marshalled manifest again")
					manifest, err := LoadYAML(text)
					Expect(err).NotTo(HaveOccurred())

					hc := m1.InstanceGroups[1].Jobs[0].Properties.Quarks.Run.HealthCheck
					Expect(hc).ToNot(BeNil())
					Expect(hc["test-server"].ReadinessProbe.Handler.Exec.Command).To(ContainElement("curl --silent --fail --head http://${HOSTNAME}:8080/health"))

					hc = manifest.InstanceGroups[1].Jobs[0].Properties.Quarks.Run.HealthCheck
					Expect(hc).ToNot(BeNil())
					Expect(hc["test-server"].ReadinessProbe.Handler.Exec.Command).To(ContainElement("curl --silent --fail --head http://${HOSTNAME}:8080/health"))
				})

				It("serializes instancegroup quarks", func() {
					m1.ApplyUpdateBlock()
					text, err := m1.Marshal()
					Expect(err).NotTo(HaveOccurred())
					By("loading marshalled manifest again")
					manifest, err := LoadYAML(text)
					Expect(err).NotTo(HaveOccurred())
					Expect(manifest.InstanceGroups).To(HaveLen(3))
					Expect(manifest.InstanceGroups[0].Properties.Quarks.RequiredService).To(BeNil())
					expectedRequireService := "bpm-affinity-bpm1"
					Expect(manifest.InstanceGroups[1].Properties.Quarks.RequiredService).To(Equal(&expectedRequireService))
					expectedRequireService = "bpm-affinity-bpm2"
					Expect(manifest.InstanceGroups[2].Properties.Quarks.RequiredService).To(Equal(&expectedRequireService))

				})
			})
		})

		Describe("GetReleaseImage", func() {
			BeforeEach(func() {
				manifest, err = env.DefaultBOSHManifest()
				Expect(err).NotTo(HaveOccurred())
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
				manifest, err = env.DefaultBOSHManifest()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error if the instance group does not exist", func() {
				_, found := manifest.InstanceGroups.InstanceGroupByName("foo")
				Expect(found).To(Equal(false))
			})

			It("returns the instance group if it exists", func() {
				ig, found := manifest.InstanceGroups.InstanceGroupByName("redis-slave")
				Expect(found).To(Equal(true))
				Expect(ig.Name).To(Equal("redis-slave"))
			})
		})

		Describe("ApplyUpdateBlock", func() {
			BeforeEach(func() {
				manifest, err = env.BOSHManifestWithUpdateSerial()
				Expect(err).NotTo(HaveOccurred())
			})

			It("calculates first instance group without dependency", func() {
				manifest.ApplyUpdateBlock()
				Expect(manifest.InstanceGroups).To(HaveLen(4))
				Expect(manifest.InstanceGroups[0].Properties.Quarks.RequiredService).To(BeNil())
			})

			It("respects serial=true on instance group as barrier", func() {
				manifest.ApplyUpdateBlock()
				Expect(manifest.InstanceGroups).To(HaveLen(4))
				expectedRequireService := "bpm-bpm1"
				Expect(manifest.InstanceGroups[1].Properties.Quarks.RequiredService).To(Equal(&expectedRequireService))
				Expect(manifest.InstanceGroups[2].Properties.Quarks.RequiredService).To(Equal(&expectedRequireService))
			})

			It("respects serial=true to wait for the predecessor", func() {
				manifest.ApplyUpdateBlock()
				Expect(manifest.InstanceGroups).To(HaveLen(4))
				expectedRequireService := "bpm-bpm3"
				Expect(manifest.InstanceGroups[3].Properties.Quarks.RequiredService).To(Equal(&expectedRequireService))
			})

			It("respects update serial in manifest", func() {
				manifestWithUpdate, err := env.BOSHManifestWithUpdateSerialInManifest()
				Expect(err).NotTo(HaveOccurred())
				manifestWithUpdate.ApplyUpdateBlock()
				Expect(manifestWithUpdate.InstanceGroups).To(HaveLen(2))
				Expect(manifestWithUpdate.InstanceGroups[0].Properties.Quarks.RequiredService).To(BeNil())
				Expect(manifestWithUpdate.InstanceGroups[1].Properties.Quarks.RequiredService).To(BeNil())
			})

			It("doesn't wait for instance groups without ports", func() {
				manifestWithUpdate, err := env.BOSHManifestWithUpdateSerialAndWithoutPorts()
				Expect(err).NotTo(HaveOccurred())
				manifestWithUpdate.ApplyUpdateBlock()
				Expect(manifestWithUpdate.InstanceGroups).To(HaveLen(3))
				expectedRequireService := "bpm-bpm1"
				Expect(manifestWithUpdate.InstanceGroups[0].Properties.Quarks.RequiredService).To(BeNil())
				Expect(manifestWithUpdate.InstanceGroups[1].Properties.Quarks.RequiredService).To(Equal(&expectedRequireService))
				Expect(manifestWithUpdate.InstanceGroups[2].Properties.Quarks.RequiredService).To(Equal(&expectedRequireService))
			})
			It("propagates global update block correctly", func() {
				manifest, err = env.BOSHManifestWithGlobalUpdateBlock()
				Expect(err).NotTo(HaveOccurred())
				manifest.ApplyUpdateBlock()
				By("propagating if ig has no update block")
				Expect(*manifest.InstanceGroups[0].Update).To(Equal(Update{
					CanaryWatchTime: "20000-1200000",
					UpdateWatchTime: "20000-1200000",
					Serial:          pointer.BoolPtr(false),
				}))
				By("retaining ig's serial configuration")
				Expect(*manifest.InstanceGroups[1].Update).To(Equal(Update{
					CanaryWatchTime: "20000-1200000",
					UpdateWatchTime: "20000-1200000",
					Serial:          pointer.BoolPtr(true),
				}))
				By("retaining ig's canaryWatchTime configuration")
				Expect(*manifest.InstanceGroups[2].Update).To(Equal(Update{
					CanaryWatchTime: "10000-9900000",
					UpdateWatchTime: "10000-9900000",
					Serial:          pointer.BoolPtr(false),
				}))
			})
		})
	})
})
