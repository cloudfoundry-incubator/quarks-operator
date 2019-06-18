package manifest

import (
	"fmt"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
)

// InstanceGroup from BOSH deployment manifest
type InstanceGroup struct {
	Name               string                 `yaml:"name"`
	Instances          int                    `yaml:"instances"`
	AZs                []string               `yaml:"azs"`
	Jobs               []Job                  `yaml:"jobs"`
	VMType             string                 `yaml:"vm_type,omitempty"`
	VMExtensions       []string               `yaml:"vm_extensions,omitempty"`
	VMResources        *VMResource            `yaml:"vm_resources"`
	Stemcell           string                 `yaml:"stemcell"`
	PersistentDisk     *int                   `yaml:"persistent_disk,omitempty"`
	PersistentDiskType string                 `yaml:"persistent_disk_type,omitempty"`
	Networks           []*Network             `yaml:"networks,omitempty"`
	Update             *Update                `yaml:"update,omitempty"`
	MigratedFrom       []*MigratedFrom        `yaml:"migrated_from,omitempty"`
	LifeCycle          string                 `yaml:"lifecycle,omitempty"`
	Properties         map[string]interface{} `yaml:"properties,omitempty"`
	Env                AgentEnv               `yaml:"env,omitempty"`
}

func (ig *InstanceGroup) jobInstances(namespace string, deploymentName string, jobName string, spec JobSpec) []JobInstance {
	var jobsInstances []JobInstance
	for i := 0; i < ig.Instances; i++ {

		// TODO: Understand whether there are negative side-effects to using this
		// default
		azs := []string{""}
		if len(ig.AZs) > 0 {
			azs = ig.AZs
		}

		for _, az := range azs {
			index := len(jobsInstances)
			name := fmt.Sprintf("%s-%s", ig.Name, jobName)
			id := fmt.Sprintf("%s-%d-%s", ig.Name, index, jobName)
			// All jobs in same instance group will use same service
			serviceName := fmt.Sprintf("%s-%s-%d", deploymentName, ig.Name, index)
			// TODO: not allowed to hardcode svc.cluster.local
			address := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)

			jobsInstances = append(jobsInstances, JobInstance{
				Address:  address,
				AZ:       az,
				ID:       id,
				Index:    index,
				Instance: i,
				Name:     name,
			})
		}
	}
	return jobsInstances
}

// VMResource from BOSH deployment manifest
type VMResource struct {
	CPU               int `yaml:"cpu"`
	RAM               int `yaml:"ram"`
	EphemeralDiskSize int `yaml:"ephemeral_disk_size"`
}

// Network from BOSH deployment manifest
type Network struct {
	Name      string   `yaml:"name"`
	StaticIps []string `yaml:"static_ips,omitempty"`
	Default   []string `yaml:"default,omitempty"`
}

// Update from BOSH deployment manifest
type Update struct {
	Canaries        int     `yaml:"canaries"`
	MaxInFlight     string  `yaml:"max_in_flight"`
	CanaryWatchTime string  `yaml:"canary_watch_time"`
	UpdateWatchTime string  `yaml:"update_watch_time"`
	Serial          bool    `yaml:"serial,omitempty"`
	VMStrategy      *string `yaml:"vm_strategy,omitempty"`
}

// MigratedFrom from BOSH deployment manifest
type MigratedFrom struct {
	Name string `yaml:"name"`
	Az   string `yaml:"az,omitempty"`
}

// IPv6 from BOSH deployment manifest
type IPv6 struct {
	Enable bool `yaml:"enable"`
}

// JobDir from BOSH deployment manifest
type JobDir struct {
	Tmpfs     *bool  `yaml:"tmpfs,omitempty"`
	TmpfsSize string `yaml:"tmpfs_size,omitempty"`
}

var (
	// LabelDeploymentName is the name of a label for the deployment name
	LabelDeploymentName = fmt.Sprintf("%s/deployment-name", apis.GroupName)
	// LabelInstanceGroupName is the name of a label for an instance group name
	LabelInstanceGroupName = fmt.Sprintf("%s/instance-group-name", apis.GroupName)
	// LabelDeploymentVersion is the name of a label for the deployment's version
	LabelDeploymentVersion = fmt.Sprintf("%s/deployment-version", apis.GroupName)
)

// AgentSettings from BOSH deployment manifest. These annotations and labels are added to kube resources
type AgentSettings struct {
	Annotations map[string]string `yaml:"annotations,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

// Set overrides labels and annotations with operator-owned metadata
func (as *AgentSettings) Set(manifestName, igName, version string) {
	if as.Labels == nil {
		as.Labels = map[string]string{}
	}
	as.Labels[LabelDeploymentName] = manifestName
	as.Labels[LabelInstanceGroupName] = igName
	as.Labels[LabelDeploymentVersion] = version
}

// Agent from BOSH deployment manifest
type Agent struct {
	Settings AgentSettings `yaml:"settings,omitempty"`
	Tmpfs    *bool         `yaml:"tmpfs,omitempty"`
}

// AgentEnvBoshConfig from BOSH deployment manifest
type AgentEnvBoshConfig struct {
	Password              string  `yaml:"password,omitempty"`
	KeepRootPassword      string  `yaml:"keep_root_password,omitempty"`
	RemoveDevTools        *bool   `yaml:"remove_dev_tools,omitempty"`
	RemoveStaticLibraries *bool   `yaml:"remove_static_libraries,omitempty"`
	SwapSize              *int    `yaml:"swap_size,omitempty"`
	IPv6                  IPv6    `yaml:"ipv6,omitempty"`
	JobDir                *JobDir `yaml:"job_dir,omitempty"`
	Agent                 Agent   `yaml:"agent,omitempty"`
}

// AgentEnv from BOSH deployment manifest
type AgentEnv struct {
	PersistentDiskFS           string             `yaml:"persistent_disk_fs,omitempty"`
	PersistentDiskMountOptions []string           `yaml:"persistent_disk_mount_options,omitempty"`
	AgentEnvBoshConfig         AgentEnvBoshConfig `yaml:"bosh,omitempty"`
}
