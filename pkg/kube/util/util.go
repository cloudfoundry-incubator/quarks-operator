package util

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// UnionMaps creates a new map with all values contained in the maps passed to this function
func UnionMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// MinInt32 returns the minimum of two int32 values
func MinInt32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

// MaxInt32 returns the maximum of two int32 values
func MaxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// ServiceName returns the service name for a deployment
func ServiceName(instanceGroupName string, deploymentName string, maxLength int) string {
	serviceName := fmt.Sprintf("%s-%s", deploymentName, names.Sanitize(instanceGroupName))
	if len(serviceName) > maxLength {
		sumHex := md5.Sum([]byte(serviceName))
		sum := hex.EncodeToString(sumHex[:])
		serviceName = fmt.Sprintf("%s-%s", serviceName[:maxLength-len(sum)-1], sum)
	}
	return serviceName
}
