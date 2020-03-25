package manifest

import (
	corev1 "k8s.io/api/core/v1"
)

// BPMResourceDisk represents a converted BPM disk to k8s resources.
type BPMResourceDisk struct {
	PersistentVolumeClaim *corev1.PersistentVolumeClaim `json:"pvc,omitempty"`
	Volume                *corev1.Volume                `json:"volume,omitempty"`
	VolumeMount           *corev1.VolumeMount           `json:"volumeMount,omitempty"`

	Filters map[string]string `json:"filters,omitempty"`
}

// BPMResourceDisks represents a slice of BPMResourceDisk.
type BPMResourceDisks []BPMResourceDisk

// MatchesFilter returns true if the disk matches the filter with one of its Filters.
func (disk *BPMResourceDisk) MatchesFilter(filterKey, filterValue string) bool {
	labelValue, exists := disk.Filters[filterKey]
	if !exists {
		return false
	}
	return labelValue == filterValue
}

// Filter filters BPMResourceDisks on its Filters.
func (disks BPMResourceDisks) Filter(filterKey, filterValue string) BPMResourceDisks {
	filtered := make(BPMResourceDisks, 0)
	for _, disk := range disks {
		if disk.MatchesFilter(filterKey, filterValue) {
			filtered = append(filtered, disk)
		}
	}
	return filtered
}

// VolumeMounts returns a slice of VolumeMount of each BPMResourceDisk contained in BPMResourceDisks.
func (disks BPMResourceDisks) VolumeMounts() []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0)
	for _, disk := range disks {
		if disk.VolumeMount != nil {
			volumeMounts = append(volumeMounts, *disk.VolumeMount)
		}
	}
	return volumeMounts
}

// Volumes returns a slice of Volume of each BPMResourceDisk contained in BPMResourceDisks.
func (disks BPMResourceDisks) Volumes() []corev1.Volume {
	volumes := make([]corev1.Volume, 0)
	for _, disk := range disks {
		if disk.Volume != nil {
			volumes = append(volumes, *disk.Volume)
		}
	}
	return volumes
}

// PVCs returns a slice of PVC of each BPMResourceDisk
func (disks BPMResourceDisks) PVCs() []corev1.PersistentVolumeClaim {
	pvcs := make([]corev1.PersistentVolumeClaim, 0)
	for _, disk := range disks {

		if disk.PersistentVolumeClaim != nil {
			pvcs = append(pvcs, *disk.PersistentVolumeClaim)
		}
	}
	return pvcs
}
