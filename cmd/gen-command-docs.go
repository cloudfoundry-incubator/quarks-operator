package main

import (
	"log"

	cmd "code.cloudfoundry.org/quarks-operator/cmd/internal"
	"github.com/spf13/cobra/doc"
)

func main() {

	cfOperatorCommand := cmd.NewCFOperatorCommand()

	err := doc.GenMarkdownTree(cfOperatorCommand, "./docs/commands/")
	if err != nil {
		log.Fatal(err)
	}
}
