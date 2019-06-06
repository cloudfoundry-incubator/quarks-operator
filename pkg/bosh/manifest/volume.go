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
)
