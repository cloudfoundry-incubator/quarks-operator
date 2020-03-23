package bpmconverter

import (
	corev1 "k8s.io/api/core/v1"
)

// BPMResourceDisk represents a converted BPM disk to k8s resources.
type BPMResourceDisk struct {
	PersistentVolumeClaim *corev1.PersistentVolumeClaim
	Volume                *corev1.Volume
	VolumeMount           *corev1.VolumeMount

	Filters map[string]string
}

// matchesFilter returns true if the disk matches the filter with one of its Filters.
func (disk *BPMResourceDisk) matchesFilter(filterKey, filterValue string) bool {
	labelValue, exists := disk.Filters[filterKey]
	if !exists {
		return false
	}
	return labelValue == filterValue
}

// BPMResourceDisks represents a slice of BPMResourceDisk.
type BPMResourceDisks []BPMResourceDisk

// filter filters BPMResourceDisks on its Filters.
func (disks BPMResourceDisks) filter(filterKey, filterValue string) BPMResourceDisks {
	filtered := make(BPMResourceDisks, 0)
	for _, disk := range disks {
		if disk.matchesFilter(filterKey, filterValue) {
			filtered = append(filtered, disk)
		}
	}
	return filtered
}

// VolumeMounts returns a slice of VolumeMount of each BPMResourceDisk contained in BPMResourceDisks.
func (disks BPMResourceDisks) volumeMounts() []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0)
	for _, disk := range disks {
		if disk.VolumeMount != nil {
			volumeMounts = append(volumeMounts, *disk.VolumeMount)
		}
	}
	return volumeMounts
}

// Volumes returns a slice of Volume of each BPMResourceDisk contained in BPMResourceDisks.
func (disks BPMResourceDisks) volumes() []corev1.Volume {
	volumes := make([]corev1.Volume, 0)
	for _, disk := range disks {
		if disk.Volume != nil {
			volumes = append(volumes, *disk.Volume)
		}
	}
	return volumes
}

// PVCs returns a slice of PVC of each BPMResourceDisk
func (disks BPMResourceDisks) pvcs() []corev1.PersistentVolumeClaim {
	pvcs := make([]corev1.PersistentVolumeClaim, 0)
	for _, disk := range disks {

		if disk.PersistentVolumeClaim != nil {
			pvcs = append(pvcs, *disk.PersistentVolumeClaim)
		}
	}
	return pvcs
}
