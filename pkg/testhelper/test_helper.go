// Package testhelper has convenience functions for returning pointers to values
package testhelper

// Int32 returns a pointer to the int32 value provided
func Int32(v int32) *int32 {
	return &v
}

// String returns a pointer to the string value provided
func String(v string) *string {
	return &v
}

// Bool returns a pointer to the bool value provided
func Bool(v bool) *bool {
	return &v
}
