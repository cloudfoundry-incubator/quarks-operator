package main

import (
	"os"

	utils "code.cloudfoundry.org/quarks-utils/pkg/cmd"

	cmd "code.cloudfoundry.org/quarks-operator/cmd/internal"
)

const (
	index = `---
title: "Quarks-operator"
linkTitle: "Quarks-operator"
weight: 20
description: >
    Quarks-operator
---
	`
)

func main() {
	docDir := os.Args[1]
	if err := utils.GenCLIDocsyMarkDown(cmd.NewCFOperatorCommand(), docDir, index); err != nil {
		panic(err)
	}
}
