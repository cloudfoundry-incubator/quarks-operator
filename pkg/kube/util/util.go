package util

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
