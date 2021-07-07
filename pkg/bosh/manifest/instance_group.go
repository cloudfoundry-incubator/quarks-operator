package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	boshnames "code.cloudfoundry.org/quarks-operator/pkg/kube/util/names"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// InstanceGroups represents a slice of pointers of InstanceGroup.
type InstanceGroups []*InstanceGroup

// InstanceGroupByName returns the instance group identified by the given name. The second return
// parameter indicates if the instance group was found.
func (instanceGroups InstanceGroups) InstanceGroupByName(name string) (*InstanceGroup, bool) {
	for _, instanceGroup := range instanceGroups {
		if instanceGroup.Name == name {
			return instanceGroup, true
		}
	}
	return nil, false
}

// InstanceGroup from BOSH deployment manifest.
type InstanceGroup struct {
	Name               string                  `json:"name"`
	Instances          int                     `json:"instances"`
	AZs                []string                `json:"azs"`
	Jobs               []Job                   `json:"jobs"`
	VMType             string                  `json:"vm_type,omitempty"`
	VMExtensions       []string                `json:"vm_extensions,omitempty"`
	VMResources        *VMResource             `json:"vm_resources"`
	Stemcell           string                  `json:"stemcell"`
	PersistentDisk     *int                    `json:"persistent_disk,omitempty"`
	PersistentDiskType string                  `json:"persistent_disk_type,omitempty"`
	Networks           []*Network              `json:"networks,omitempty"`
	Update             *Update                 `json:"update,omitempty"`
	MigratedFrom       []*MigratedFrom         `json:"migrated_from,omitempty"`
	LifeCycle          InstanceGroupType       `json:"lifecycle,omitempty"`
	Properties         InstanceGroupProperties `json:"properties,omitempty"`
	Env                AgentEnv                `json:"env,omitempty"`
}

// InstanceGroupQuarks represents the quark property of a InstanceGroup
type InstanceGroupQuarks struct {
	RequiredService *string `json:"required_service,omitempty" mapstructure:"required_service"`
}

// InstanceGroupProperties represents the properties map of a InstanceGroup
type InstanceGroupProperties struct {
	Properties map[string]interface{}
	Quarks     InstanceGroupQuarks
}

func copy(source map[string]interface{}) map[string]interface{} {
	newMap := make(map[string]interface{})
	for key, value := range source {
		newMap[key] = value
	}
	return newMap
}

// MarshalJSON is implemented to support inlining Properties
func (p *InstanceGroupProperties) MarshalJSON() ([]byte, error) {
	properties := copy(p.Properties)
	properties["quarks"] = p.Quarks
	return json.Marshal(properties)
}

// UnmarshalJSON is implemented to support inlining properties
func (p *InstanceGroupProperties) UnmarshalJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	err := d.Decode(&p.Properties)
	if err != nil {
		return err
	}
	if p.Properties != nil {

		quarks, ok := p.Properties["quarks"]
		if ok {
			if err := mapstructure.Decode(quarks, &p.Quarks); err != nil {
				return errors.Wrapf(err, "failed to quarks properties from instance group")
			}
			delete(p.Properties, "quarks")
		}
	}
	return nil
}

// NameSanitized returns the sanitized instance group name.
func (ig *InstanceGroup) NameSanitized() string {
	return names.Sanitize(ig.Name)
}

// IsErrand returns true if the  instance group is any kind of BOSH errand
func (ig *InstanceGroup) IsErrand() bool {
	return ig.LifeCycle == IGTypeErrand || ig.LifeCycle == IGTypeAutoErrand
}

// IndexedServiceName constructs an indexed service name. It's used to construct the service
// names other than the headless service.
func (ig *InstanceGroup) IndexedServiceName(index int, azIndex int, azName string) string {
	sn := boshnames.TruncatedServiceName(ig.Name, 53)
	if azIndex > -1 {
		return fmt.Sprintf("%s-%s-%d-%d", sn, azName, azIndex, index)
	}
	return fmt.Sprintf("%s-%d", sn, index)
}

func (ig *InstanceGroup) newJobInstances(
	jobName string,
	initialRollout bool,
) []JobInstance {
	if len(ig.AZs) > 0 {
		return ig.jobInstancesAZ(jobName, initialRollout)
	}
	return ig.jobInstances(jobName, initialRollout)
}

func (ig *InstanceGroup) jobInstances(
	jobName string,
	initialRollout bool,
) []JobInstance {

	bootstrapIndex := 0
	if !initialRollout {
		bootstrapIndex = ig.Instances - 1
	}

	var jobsInstances []JobInstance
	for i := 0; i < ig.Instances; i++ {
		igName := ig.NameSanitized()

		//specIndex := names.SpecIndex(azIndex+1, i))

		jobsInstances = append(jobsInstances, JobInstance{
			Address:   ig.IndexedServiceName(i, -1, ""),
			AZ:        "",
			Bootstrap: i == bootstrapIndex,
			Index:     i,
			Instance:  i,
			Name:      fmt.Sprintf("%s-%s", igName, jobName),
			ID:        fmt.Sprintf("%s-%d", igName, i),
		})
	}
	return jobsInstances
}

func (ig *InstanceGroup) jobInstancesAZ(
	jobName string,
	initialRollout bool,
) []JobInstance {

	bootstrapIndex := 0
	// TODO why does this switch from first AZ to last?
	if !initialRollout {
		bootstrapIndex = ig.Instances*len(ig.AZs) - 1
	}

	var jobsInstances []JobInstance
	for azIndex, az := range ig.AZs {
		for i := 0; i < ig.Instances; i++ {
			igName := ig.NameSanitized()
			index := len(jobsInstances)

			jobsInstances = append(jobsInstances, JobInstance{
				Address:   ig.IndexedServiceName(i, azIndex, az),
				AZ:        az,
				Bootstrap: index == bootstrapIndex,
				Index:     index,
				Instance:  i,
				Name:      fmt.Sprintf("%s-%s", igName, jobName),
				ID:        fmt.Sprintf("%s-%s-%d-%d", igName, az, azIndex, index%ig.Instances),
			})
		}
	}
	return jobsInstances
}

// ServicePorts returns the service ports used by this instance group
func (ig *InstanceGroup) ServicePorts() []corev1.ServicePort {
	// Collect ports to be exposed for each job
	ports := []corev1.ServicePort{}
	for _, job := range ig.Jobs {
		for _, port := range job.Properties.Quarks.Ports {
			ports = append(ports, corev1.ServicePort{
				Name:     port.Name,
				Protocol: corev1.Protocol(port.Protocol),
				Port:     int32(port.Internal),
			})
		}
	}
	return ports
}

// VMResource from BOSH deployment manifest.
type VMResource struct {
	CPU               int `json:"cpu"`
	RAM               int `json:"ram"`
	EphemeralDiskSize int `json:"ephemeral_disk_size"`
}

// Network from BOSH deployment manifest.
type Network struct {
	Name      string   `json:"name"`
	StaticIps []string `json:"static_ips,omitempty"`
	Default   []string `json:"default,omitempty"`
}

// Update from BOSH deployment manifest.
type Update struct {
	Canaries        int     `json:"canaries"`
	MaxInFlight     string  `json:"max_in_flight"`
	CanaryWatchTime string  `json:"canary_watch_time"`
	UpdateWatchTime string  `json:"update_watch_time"`
	Serial          *bool   `json:"serial,omitempty"` // must be pointer, because otherwise default is false
	VMStrategy      *string `json:"vm_strategy,omitempty"`
}

// MigratedFrom from BOSH deployment manifest.
type MigratedFrom struct {
	Name string `json:"name"`
	Az   string `json:"az,omitempty"`
}

// IPv6 from BOSH deployment manifest.
type IPv6 struct {
	Enable bool `json:"enable"`
}

// JobDir from BOSH deployment manifest.
type JobDir struct {
	Tmpfs     *bool  `json:"tmpfs,omitempty"`
	TmpfsSize string `json:"tmpfs_size,omitempty"`
}

// OpsPatch represents a Json patch that can be performed
// on an Instance Group or BPM properties yaml file
type OpsPatch struct {
	Type  string      `json:"type,omitempty"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// OpsPatches is a list of ops files
type OpsPatches []OpsPatch

// Bytes returns the yaml equivalent of the ops patch
func (o *OpsPatches) Bytes() ([]byte, error) {
	return yaml.Marshal(o)
}

// PreRenderOps contains ops files for BPM and Instance Groups
// yaml files
type PreRenderOps struct {
	BPM           OpsPatches `json:"bpm,omitempty"`
	InstanceGroup OpsPatches `json:"instanceGroup,omitempty"`
}

// AgentSettings from BOSH deployment manifest,
// '<instance-group>.env.bosh.agent.settings'.
// These annotations and labels are added to kube resources.
// Affinity & tolerations are added into the pod's definition.
type AgentSettings struct {
	Annotations                   map[string]string             `json:"annotations,omitempty"`
	Labels                        map[string]string             `json:"labels,omitempty"`
	Affinity                      *corev1.Affinity              `json:"affinity,omitempty"`
	DisableLogSidecar             bool                          `json:"disable_log_sidecar,omitempty" yaml:"disable_log_sidecar,omitempty"`
	ServiceAccountName            string                        `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	AutomountServiceAccountToken  *bool                         `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	ImagePullSecrets              []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	Tolerations                   []corev1.Toleration           `json:"tolerations,omitempty"`
	EphemeralAsPVC                bool                          `json:"ephemeralAsPVC,omitempty"`
	Disks                         Disks                         `json:"disks,omitempty"`
	JobBackoffLimit               *int32                        `json:"jobBackoffLimit,omitempty"`
	PreRenderOps                  *PreRenderOps                 `json:"preRenderOps,omitempty"`
	InjectReplicasEnv             *bool                         `json:"injectReplicasEnv,omitempty"`
	TerminationGracePeriodSeconds *int64                        `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	DNS                           string                        `json:"dns,omitempty"`
}

// Set overrides labels and annotations with operator-owned metadata.
func (as *AgentSettings) Set(manifestName, igName, version string) {
	if as.Labels == nil {
		as.Labels = map[string]string{}
	}
	as.Labels[bdv1.LabelDeploymentName] = manifestName
	as.Labels[bdv1.LabelInstanceGroupName] = igName
	as.Labels[bdv1.LabelDeploymentVersion] = version
}

// Agent from BOSH deployment manifest.
type Agent struct {
	Settings AgentSettings `json:"settings,omitempty"`
	Tmpfs    *bool         `json:"tmpfs,omitempty"`
}

// AgentEnvBoshConfig contains supported settings from the
// <instance-group>.env.bosh hash of the BOSH deployment manifest.
type AgentEnvBoshConfig struct {
	Password              string  `json:"password,omitempty"`
	KeepRootPassword      string  `json:"keep_root_password,omitempty"`
	RemoveDevTools        *bool   `json:"remove_dev_tools,omitempty"`
	RemoveStaticLibraries *bool   `json:"remove_static_libraries,omitempty"`
	SwapSize              *int    `json:"swap_size,omitempty"`
	IPv6                  IPv6    `json:"ipv6,omitempty"`
	JobDir                *JobDir `json:"job_dir,omitempty"`
	Agent                 Agent   `json:"agent,omitempty"`
}

// AgentEnv from BOSH deployment manifest.
type AgentEnv struct {
	PersistentDiskFS           string             `json:"persistent_disk_fs,omitempty"`
	PersistentDiskMountOptions []string           `json:"persistent_disk_mount_options,omitempty"`
	AgentEnvBoshConfig         AgentEnvBoshConfig `json:"bosh,omitempty"`
}
