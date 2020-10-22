module code.cloudfoundry.org/quarks-operator

require (
	code.cloudfoundry.org/quarks-job v1.0.206
	code.cloudfoundry.org/quarks-secret v1.0.746
	code.cloudfoundry.org/quarks-statefulset v0.0.2
	code.cloudfoundry.org/quarks-utils v0.0.2-0.20201020085845-6575bfd4b208
	github.com/SUSE/go-patch v0.3.0
	github.com/bmatcuk/doublestar v1.1.1 // indirect
	github.com/charlievieth/fs v0.0.0-20170613215519-7dc373669fa1 // indirect
	github.com/cloudfoundry/bosh-cli v5.4.0+incompatible
	github.com/cloudfoundry/bosh-utils v0.0.0-20190206192830-9a0affed2bf1 // indirect
	github.com/cppforlife/go-patch v0.2.0 // indirect
	github.com/daaku/go.zipexe v1.0.1 // indirect
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-logr/logr v0.2.0
	github.com/go-test/deep v1.0.7
	github.com/gonvenience/bunt v1.1.3
	github.com/hpcloud/tail v1.0.0
	github.com/imdario/mergo v0.3.11
	github.com/mattn/go-isatty v0.0.11 // indirect
	github.com/mitchellh/mapstructure v1.3.3
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/pkg/errors v0.9.1
	github.com/spf13/afero v1.4.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/viovanov/bosh-template-go v0.0.0-20200416144406-32ddfa4afdb0
	go.uber.org/zap v1.16.0
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	gomodules.xyz/jsonpatch/v2 v2.0.1
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.3
	k8s.io/apiextensions-apiserver v0.18.9
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/yaml v1.2.0
)

go 1.15
