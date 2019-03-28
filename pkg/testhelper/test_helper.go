// Package testhelper has convenience functions for tests, like returning
// pointers to values.
package testhelper

import "encoding/json"

// IndentedJSON returns data structure pretty printed as JSON
func IndentedJSON(data interface{}) string {
	txt, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "failed to marshal JSON"
	}

	return string(txt)
}
