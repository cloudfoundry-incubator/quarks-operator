# Releasing

We're releasing based on tags, which contain our version number. The format is 'v0.0.0'.
The release title will be set to this version.

The CI pipeline has a 'release' job, which will update the release on Github.
That job triggers itself, when a draft release is created.

## Create new release pipeline

We release from release-branches. Each maintained release has a separate pipeline in Concourse.
To create a new pipeline run this in the CI repository:

```shell
cd pipelines/cf-operator-release
./configure.sh CFO v0.4.x v0.4
```

Where `CFO` is your concourse target and `v0.4.x` is the name of the branch.
The last argument, `v0.4` is used to filter Github tags, which belong to the release.

This allows a separate Github branch and Concourse pipeline for each major version.
Within those pipelines, releases can be built from minor versions.

## Create a new release

After completion, the pipeline will create several artifacts:

* helm chart on S3
* helm chart in our repo at https://cloudfoundry-incubator.github.io/quarks-helm/
* cf-operator binary on S3
* docker image of the operator on dockerhub

Running the 'release' job will take the latest artificats, which passed through the pipeline and add them to the Github release:

* to the body
* as Github assets for downloading

The version numbers (`v0.0.0-<number-of-commits>.<commit-SHA>`) of these assets are taken from the info on S3.
They have to match the Github tag, else the release job will fail.
The assets will be copied into a 'release' folder on S3.

The docker image is only referenced from the helm chart and not mentioned in the release, though.

## Checklist

### Major Release

1. Create version branch
1. Create a new release pipeline for that branch
1. Unpause pipeline
1. Continue with "Minor Bump"

### Minor Bump

1. Tag commit with new version
1. Push commit
1. Wait for commit to pass release pipeline, 'publish' needs to create the binary and helm chart, before the 'release' job can run
1. Create a draft Github release for that tag, 'release' job triggers
1. Wait for 'release' job to finish on Concourse
1. Edit the draft release on Github and publish it

Try not to push to the pipeline again, until step 4 is completed. The 'release' job will always take the most recent artifacts from S3. Maybe pause the 'publish' job manually to avoid accidental updates.
