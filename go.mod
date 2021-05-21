module code.cloudfoundry.org/quarks-operator

require (
	code.cloudfoundry.org/quarks-job v1.0.218
	code.cloudfoundry.org/quarks-secret v1.0.760
	code.cloudfoundry.org/quarks-statefulset v1.0.1319
	code.cloudfoundry.org/quarks-utils v0.0.3-0.20210303091853-3b41f4b87e33
	github.com/SUSE/go-patch v0.3.0
	github.com/bmatcuk/doublestar v1.1.1 // indirect
	github.com/charlievieth/fs v0.0.0-20170613215519-7dc373669fa1 // indirect
	github.com/cloudfoundry/bosh-cli v5.4.0+incompatible
	github.com/cloudfoundry/bosh-utils v0.0.0-20190206192830-9a0affed2bf1 // indirect
	github.com/cppforlife/go-patch v0.2.0 // indirect
	github.com/daaku/go.zipexe v1.0.1 // indirect
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-logr/logr v0.3.0
	github.com/go-test/deep v1.0.7
	github.com/gonvenience/bunt v1.1.4
	github.com/hpcloud/tail v1.0.0
	github.com/imdario/mergo v0.3.11
	github.com/mattn/go-isatty v0.0.11 // indirect
	github.com/mitchellh/mapstructure v1.3.3
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/onsi/ginkgo v1.16.0
	github.com/onsi/gomega v1.10.3
	github.com/pkg/errors v0.9.1
	github.com/spf13/afero v1.4.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/viovanov/bosh-template-go v0.0.0-20200416144406-32ddfa4afdb0
	go.uber.org/zap v1.16.0
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	gomodules.xyz/jsonpatch/v2 v2.1.0
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.20.4
	k8s.io/apiextensions-apiserver v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.20.4
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/controller-runtime v0.8.2
	sigs.k8s.io/yaml v1.2.0
)

go 1.16
