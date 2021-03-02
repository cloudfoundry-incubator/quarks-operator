package testing

import (
	"encoding/json"
	"fmt"
)

// JSONDump pretty prints a struct
func JSONDump(props interface{}) {
	js, _ := json.MarshalIndent(props, "", "  ")
	fmt.Print(string(js))
}

// PrettyPrint returns the interface as indented JSON
func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}
