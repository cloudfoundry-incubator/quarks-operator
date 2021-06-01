package manifest

import (
	corev1 "k8s.io/api/core/v1"
)

// Disk represents a converted BPM disk to k8s resources.
type Disk struct {
	PersistentVolumeClaim *corev1.PersistentVolumeClaim `json:"pvc,omitempty"`
	Volume                *corev1.Volume                `json:"volume,omitempty"`
	VolumeMount           *corev1.VolumeMount           `json:"volumeMount,omitempty"`

	Filters map[string]string `json:"filters,omitempty"`
}

// Disks represents a slice of BPMResourceDisk.
// Part of the BOSH manifest at '<instance-group>.env.bosh.agent.settings.disks'.
type Disks []Disk

// MatchesFilter returns true if the disk matches the filter with one of its Filters.
func (disk *Disk) MatchesFilter(filterKey, filterValue string) bool {
	labelValue, exists := disk.Filters[filterKey]
	if !exists {
		return false
	}
	return labelValue == filterValue
}

// Filter filters BPMResourceDisks on its Filters.
func (disks Disks) Filter(filterKey, filterValue string) Disks {
	filtered := make(Disks, 0)
	for _, disk := range disks {
		if disk.MatchesFilter(filterKey, filterValue) {
			filtered = append(filtered, disk)
		}
	}
	return filtered
}

// VolumeMounts returns a slice of VolumeMount of each BPMResourceDisk contained in BPMResourceDisks.
func (disks Disks) VolumeMounts() []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0)
	for _, disk := range disks {
		if disk.VolumeMount != nil {
			volumeMounts = append(volumeMounts, *disk.VolumeMount)
		}
	}
	return volumeMounts
}

// Volumes returns a slice of Volume of each BPMResourceDisk contained in BPMResourceDisks.
func (disks Disks) Volumes() []corev1.Volume {
	volumes := make([]corev1.Volume, 0)
	for _, disk := range disks {
		if disk.Volume != nil {
			volumes = append(volumes, *disk.Volume)
		}
	}
	return volumes
}

// PVCs returns a slice of PVC of each BPMResourceDisk
func (disks Disks) PVCs() []corev1.PersistentVolumeClaim {
	pvcs := make([]corev1.PersistentVolumeClaim, 0)
	for _, disk := range disks {

		if disk.PersistentVolumeClaim != nil {
			pvcs = append(pvcs, *disk.PersistentVolumeClaim)
		}
	}
	return pvcs
}

// BPMMounts returns the volume mounts for the containers
func (disks Disks) BPMMounts() (*corev1.VolumeMount, *corev1.VolumeMount) {
	var ephemeralMount *corev1.VolumeMount
	ephemeralDisks := disks.Filter("ephemeral", "true")
	if len(ephemeralDisks) > 0 {
		ephemeralMount = ephemeralDisks[0].VolumeMount
	}

	var persistentDiskMount *corev1.VolumeMount
	persistentDiskDisks := disks.Filter("persistent", "true")
	if len(persistentDiskDisks) > 0 {
		persistentDiskMount = persistentDiskDisks[0].VolumeMount
	}

	return ephemeralMount, persistentDiskMount
}
