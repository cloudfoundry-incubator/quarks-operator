package names

import (
	"errors"
	"path"
)

// GetDockerSourceName returns the name of the docker image
// More info:
//   * https://kubernetes.io/docs/concepts/containers/images
//   * <hub-user>/<repo-name>[:<tag>]
//   * ACCOUNT.dkr.ecr.REGION.amazonaws.com/imagename:tag
//
// prefix: [<host>[:<port>]/][<org>/]
// repo: <name>
// tag :[:<tag>]
func GetDockerSourceName(prefix, repo, tag string) (string, error) {
	source := ""
	if repo == "" {
		return "", errors.New("repository is mandatory")
	}
	source = path.Join(prefix, repo)
	if tag != "" {
		source = source + ":" + tag
	}
	return source, nil
}
