package manifest

const (
	// VolumeRenderingDataName is the volume name for the rendering data.
	VolumeRenderingDataName = "rendering-data"
	// VolumeRenderingDataMountPath is the mount path for the rendering data.
	VolumeRenderingDataMountPath = "/var/vcap/all-releases"

	// VolumeJobsDirName is the volume name for the jobs directory.
	VolumeJobsDirName = "jobs-dir"
	// VolumeJobsDirMountPath is the mount path for the jobs directory.
	VolumeJobsDirMountPath = "/var/vcap/jobs"

	// VolumeJobsSrcDirMountPath is the mount path for the jobs-src directory.
	VolumeJobsSrcDirMountPath = "/var/vcap/jobs-src"

	// VolumeDataDirName is the volume name for the data directory.
	VolumeDataDirName = "data-dir"
	// VolumeDataDirMountPath is the mount path for the data directory.
	VolumeDataDirMountPath = "/var/vcap/data"

	// VolumeSysDirName is the volume name for the sys directory.
	VolumeSysDirName = "sys-dir"
	// VolumeSysDirMountPath is the mount path for the sys directory.
	VolumeSysDirMountPath = "/var/vcap/sys"

	// VolumeStoreDirName is the volume name for the store directory.
	VolumeStoreDirName = "store-dir"
	// VolumeStoreDirMountPath is the mount path for the store directory.
	VolumeStoreDirMountPath = "/var/vcap/store"

	// VolumeEphemeralDirName is the volume name for the ephemeral disk directory.
	VolumeEphemeralDirName = "bpm-ephemeral-disk"
	// VolumeEphemeralDirMountPath is the mount path for the ephemeral directory.
	VolumeEphemeralDirMountPath = "/var/vcap/data/"

	// VolumePersistentDirName is the volume name for the persistent disk directory.
	VolumePersistentDirName = "bpm-persistent-disk"
	// VolumePersistentDirMountPath is the mount path for the persistent directory.
	VolumePersistentDirMountPath = "/var/vcap/store/"

	// AdditionalVolume helps in building an additional volume name together with
	// the index under the additional_volumes bpm list inside the bpm process schema
	AdditionalVolume = "bpm-additional-volume"

	// AdditionalVolumesRegexValidation ensures only a valid path is defined
	// under the additional_volumes bpm list inside the bpm process schema
	AdditionalVolumesRegexValidation = "((/var/vcap/data/.+)|(/var/vcap/store/.+)|(/var/vcap/sys/run/.+))"
)
