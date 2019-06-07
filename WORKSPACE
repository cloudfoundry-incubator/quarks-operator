workspace(name = "org_cloudfoundry_code_cf_operator")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "aed1c249d4ec8f703edddf35cbe9dfaca0b5f5ea6e4cd9e83e99f3b0d1136c3d",
    strip_prefix = "rules_docker-0.7.0",
    urls = ["https://github.com/bazelbuild/rules_docker/archive/v0.7.0.tar.gz"],
)

http_archive(
    name = "io_bazel_rules_go",
    urls = ["https://github.com/bazelbuild/rules_go/releases/download/0.18.5/rules_go-0.18.5.tar.gz"],
    sha256 = "a82a352bffae6bee4e95f68a8d80a70e87f42c4741e6a448bec11998fcc82329",
)

http_archive(
    name = "bazel_gazelle",
    urls = ["https://github.com/bazelbuild/bazel-gazelle/releases/download/0.17.0/bazel-gazelle-0.17.0.tar.gz"],
    sha256 = "3c681998538231a2d24d0c07ed5a7658cb72bfb5fd4bf9911157c0e9ac6a2687",
)

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
)

container_pull(
    name = "cf_operator_base",
    registry = "index.docker.io",
    repository = "thulioassis/cf-operator-base",
    digest = "sha256:b5eef819eedf3ae118a93b9aa8cea783900b2d4b4d49fc4b6def84d6698ada00",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies()

go_repository(
    name = "com_github_appscode_jsonpatch",
    build_file_proto_mode = "disable_global",
    commit = "7c0e3b262f30",
    importpath = "github.com/appscode/jsonpatch",
)

go_repository(
    name = "com_github_beorn7_perks",
    build_file_proto_mode = "disable_global",
    commit = "3a771d992973",
    importpath = "github.com/beorn7/perks",
)

go_repository(
    name = "com_github_bmatcuk_doublestar",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/bmatcuk/doublestar",
    tag = "v1.1.1",
)

go_repository(
    name = "com_github_burntsushi_toml",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/BurntSushi/toml",
    tag = "v0.3.1",
)

go_repository(
    name = "com_github_charlievieth_fs",
    build_file_proto_mode = "disable_global",
    commit = "7dc373669fa1",
    importpath = "github.com/charlievieth/fs",
)

go_repository(
    name = "com_github_cloudflare_cfssl",
    build_file_proto_mode = "disable_global",
    commit = "ea4033a214e7",
    importpath = "github.com/cloudflare/cfssl",
)

go_repository(
    name = "com_github_cloudfoundry_bosh_cli",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/cloudfoundry/bosh-cli",
    tag = "v5.4.0",
)

go_repository(
    name = "com_github_cloudfoundry_bosh_utils",
    build_file_proto_mode = "disable_global",
    commit = "9a0affed2bf1",
    importpath = "github.com/cloudfoundry/bosh-utils",
)

go_repository(
    name = "com_github_cppforlife_go_patch",
    build_file_proto_mode = "disable_global",
    commit = "250da0e0e68c",
    importpath = "github.com/cppforlife/go-patch",
)

go_repository(
    name = "com_github_cpuguy83_go_md2man",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/cpuguy83/go-md2man",
    tag = "v1.0.10",
)

go_repository(
    name = "com_github_davecgh_go_spew",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/davecgh/go-spew",
    tag = "v1.1.1",
)

go_repository(
    name = "com_github_dchest_uniuri",
    build_file_proto_mode = "disable_global",
    commit = "8902c56451e9",
    importpath = "github.com/dchest/uniuri",
)

go_repository(
    name = "com_github_evanphx_json_patch",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/evanphx/json-patch",
    tag = "v4.0.0",
)

go_repository(
    name = "com_github_fsnotify_fsnotify",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/fsnotify/fsnotify",
    tag = "v1.4.7",
)

go_repository(
    name = "com_github_ghodss_yaml",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/ghodss/yaml",
    tag = "v1.0.0",
)

go_repository(
    name = "com_github_go_logr_logr",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/go-logr/logr",
    tag = "v0.1.0",
)

go_repository(
    name = "com_github_go_logr_zapr",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/go-logr/zapr",
    tag = "v0.1.0",
)

go_repository(
    name = "com_github_go_sql_driver_mysql",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/go-sql-driver/mysql",
    tag = "v1.4.1",
)

go_repository(
    name = "com_github_gogo_protobuf",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/gogo/protobuf",
    tag = "v1.1.1",
)

go_repository(
    name = "com_github_golang_groupcache",
    build_file_proto_mode = "disable_global",
    commit = "6f2cf27854a4",
    importpath = "github.com/golang/groupcache",
)

go_repository(
    name = "com_github_golang_protobuf",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/golang/protobuf",
    tag = "v1.2.0",
)

go_repository(
    name = "com_github_google_btree",
    build_file_proto_mode = "disable_global",
    commit = "4030bb1f1f0c",
    importpath = "github.com/google/btree",
)

go_repository(
    name = "com_github_google_certificate_transparency_go",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/google/certificate-transparency-go",
    tag = "v1.0.21",
)

go_repository(
    name = "com_github_google_gofuzz",
    build_file_proto_mode = "disable_global",
    commit = "24818f796faf",
    importpath = "github.com/google/gofuzz",
)

go_repository(
    name = "com_github_google_uuid",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/google/uuid",
    tag = "v1.0.0",
)

go_repository(
    name = "com_github_googleapis_gnostic",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/googleapis/gnostic",
    tag = "v0.2.0",
)

go_repository(
    name = "com_github_gregjones_httpcache",
    build_file_proto_mode = "disable_global",
    commit = "9cad4c3443a7",
    importpath = "github.com/gregjones/httpcache",
)

go_repository(
    name = "com_github_hashicorp_golang_lru",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/hashicorp/golang-lru",
    tag = "v0.5.0",
)

go_repository(
    name = "com_github_hashicorp_hcl",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/hashicorp/hcl",
    tag = "v1.0.0",
)

go_repository(
    name = "com_github_hpcloud_tail",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/hpcloud/tail",
    tag = "v1.0.0",
)

go_repository(
    name = "com_github_imdario_mergo",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/imdario/mergo",
    tag = "v0.3.6",
)

go_repository(
    name = "com_github_inconshreveable_mousetrap",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/inconshreveable/mousetrap",
    tag = "v1.0.0",
)

go_repository(
    name = "com_github_jmoiron_sqlx",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/jmoiron/sqlx",
    tag = "v1.2.0",
)

go_repository(
    name = "com_github_json_iterator_go",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/json-iterator/go",
    tag = "v1.1.5",
)

go_repository(
    name = "com_github_kisielk_sqlstruct",
    build_file_proto_mode = "disable_global",
    commit = "648daed35d49",
    importpath = "github.com/kisielk/sqlstruct",
)

go_repository(
    name = "com_github_lib_pq",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/lib/pq",
    tag = "v1.0.0",
)

go_repository(
    name = "com_github_magiconair_properties",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/magiconair/properties",
    tag = "v1.8.0",
)

go_repository(
    name = "com_github_mattn_go_sqlite3",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/mattn/go-sqlite3",
    tag = "v1.10.0",
)

go_repository(
    name = "com_github_matttproud_golang_protobuf_extensions",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/matttproud/golang_protobuf_extensions",
    tag = "v1.0.1",
)

go_repository(
    name = "com_github_mitchellh_mapstructure",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/mitchellh/mapstructure",
    tag = "v1.1.2",
)

go_repository(
    name = "com_github_modern_go_concurrent",
    build_file_proto_mode = "disable_global",
    commit = "bacd9c7ef1dd",
    importpath = "github.com/modern-go/concurrent",
)

go_repository(
    name = "com_github_modern_go_reflect2",
    build_file_proto_mode = "disable_global",
    commit = "4b7aa43c6742",
    importpath = "github.com/modern-go/reflect2",
)

go_repository(
    name = "com_github_nu7hatch_gouuid",
    build_file_proto_mode = "disable_global",
    commit = "179d4d0c4d8d",
    importpath = "github.com/nu7hatch/gouuid",
)

go_repository(
    name = "com_github_onsi_ginkgo",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/onsi/ginkgo",
    tag = "v1.6.0",
)

go_repository(
    name = "com_github_onsi_gomega",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/onsi/gomega",
    tag = "v1.4.2",
)

go_repository(
    name = "com_github_pborman_uuid",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/pborman/uuid",
    tag = "v1.2.0",
)

go_repository(
    name = "com_github_pelletier_go_toml",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/pelletier/go-toml",
    tag = "v1.2.0",
)

go_repository(
    name = "com_github_peterbourgon_diskv",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/peterbourgon/diskv",
    tag = "v2.0.1",
)

go_repository(
    name = "com_github_pkg_errors",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/pkg/errors",
    tag = "v0.8.1",
)

go_repository(
    name = "com_github_pmezard_go_difflib",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/pmezard/go-difflib",
    tag = "v1.0.0",
)

go_repository(
    name = "com_github_prometheus_client_golang",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/prometheus/client_golang",
    tag = "v0.9.2",
)

go_repository(
    name = "com_github_prometheus_client_model",
    build_file_proto_mode = "disable_global",
    commit = "5c3871d89910",
    importpath = "github.com/prometheus/client_model",
)

go_repository(
    name = "com_github_prometheus_common",
    build_file_proto_mode = "disable_global",
    commit = "4724e9255275",
    importpath = "github.com/prometheus/common",
)

go_repository(
    name = "com_github_prometheus_procfs",
    build_file_proto_mode = "disable_global",
    commit = "1dc9a6cbc91a",
    importpath = "github.com/prometheus/procfs",
)

go_repository(
    name = "com_github_russross_blackfriday",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/russross/blackfriday",
    tag = "v1.5.2",
)

go_repository(
    name = "com_github_spf13_afero",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/spf13/afero",
    tag = "v1.1.2",
)

go_repository(
    name = "com_github_spf13_cast",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/spf13/cast",
    tag = "v1.3.0",
)

go_repository(
    name = "com_github_spf13_cobra",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/spf13/cobra",
    tag = "v0.0.3",
)

go_repository(
    name = "com_github_spf13_jwalterweatherman",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/spf13/jwalterweatherman",
    tag = "v1.0.0",
)

go_repository(
    name = "com_github_spf13_pflag",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/spf13/pflag",
    tag = "v1.0.3",
)

go_repository(
    name = "com_github_spf13_viper",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/spf13/viper",
    tag = "v1.2.1",
)

go_repository(
    name = "com_github_stretchr_objx",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/stretchr/objx",
    tag = "v0.1.0",
)

go_repository(
    name = "com_github_stretchr_testify",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/stretchr/testify",
    tag = "v1.3.0",
)

go_repository(
    name = "com_github_viovanov_bosh_template_go",
    build_file_proto_mode = "disable_global",
    commit = "753a0fd8d6cb",
    importpath = "github.com/viovanov/bosh-template-go",
)

go_repository(
    name = "in_gopkg_check_v1",
    build_file_proto_mode = "disable_global",
    commit = "20d25e280405",
    importpath = "gopkg.in/check.v1",
)

go_repository(
    name = "in_gopkg_fsnotify_v1",
    build_file_proto_mode = "disable_global",
    importpath = "gopkg.in/fsnotify.v1",
    tag = "v1.4.7",
)

go_repository(
    name = "in_gopkg_inf_v0",
    build_file_proto_mode = "disable_global",
    importpath = "gopkg.in/inf.v0",
    tag = "v0.9.1",
)

go_repository(
    name = "in_gopkg_tomb_v1",
    build_file_proto_mode = "disable_global",
    commit = "dd632973f1e7",
    importpath = "gopkg.in/tomb.v1",
)

go_repository(
    name = "in_gopkg_yaml_v2",
    build_file_proto_mode = "disable_global",
    importpath = "gopkg.in/yaml.v2",
    tag = "v2.2.2",
)

go_repository(
    name = "io_k8s_api",
    build_file_proto_mode = "disable_global",
    commit = "5cb15d344471",
    importpath = "k8s.io/api",
)

go_repository(
    name = "io_k8s_apiextensions_apiserver",
    build_file_proto_mode = "disable_global",
    commit = "007dc40467c5",
    importpath = "k8s.io/apiextensions-apiserver",
)

go_repository(
    name = "io_k8s_apimachinery",
    build_file_proto_mode = "disable_global",
    commit = "86fb29eff628",
    importpath = "k8s.io/apimachinery",
)

go_repository(
    name = "io_k8s_client_go",
    importpath = "k8s.io/client-go",
    tag = "kubernetes-1.13.6",
    build_file_proto_mode = "disable_global",
)

go_repository(
    name = "io_k8s_klog",
    build_file_proto_mode = "disable_global",
    importpath = "k8s.io/klog",
    tag = "v0.1.0",
)

go_repository(
    name = "io_k8s_kube_openapi",
    build_file_proto_mode = "disable_global",
    commit = "96e8bb74ecdd",
    importpath = "k8s.io/kube-openapi",
)

go_repository(
    name = "io_k8s_sigs_controller_runtime",
    build_extra_args = ["-exclude=vendor"],
    build_file_proto_mode = "disable_global",
    importpath = "sigs.k8s.io/controller-runtime",
    tag = "v0.1.10",
)

go_repository(
    name = "io_k8s_sigs_testing_frameworks",
    build_file_proto_mode = "disable_global",
    importpath = "sigs.k8s.io/testing_frameworks",
    tag = "v0.1.1",
)

go_repository(
    name = "io_k8s_sigs_yaml",
    build_file_proto_mode = "disable_global",
    importpath = "sigs.k8s.io/yaml",
    tag = "v1.1.0",
)

go_repository(
    name = "org_golang_google_appengine",
    build_file_proto_mode = "disable_global",
    importpath = "google.golang.org/appengine",
    tag = "v1.2.0",
)

go_repository(
    name = "org_golang_x_crypto",
    build_file_proto_mode = "disable_global",
    commit = "c2843e01d9a2",
    importpath = "golang.org/x/crypto",
)

go_repository(
    name = "org_golang_x_net",
    build_file_proto_mode = "disable_global",
    commit = "d8887717615a",
    importpath = "golang.org/x/net",
)

go_repository(
    name = "org_golang_x_oauth2",
    build_file_proto_mode = "disable_global",
    commit = "9dcd33a902f4",
    importpath = "golang.org/x/oauth2",
)

go_repository(
    name = "org_golang_x_sync",
    build_file_proto_mode = "disable_global",
    commit = "42b317875d0f",
    importpath = "golang.org/x/sync",
)

go_repository(
    name = "org_golang_x_sys",
    build_file_proto_mode = "disable_global",
    commit = "d0b11bdaac8a",
    importpath = "golang.org/x/sys",
)

go_repository(
    name = "org_golang_x_text",
    build_file_proto_mode = "disable_global",
    importpath = "golang.org/x/text",
    tag = "v0.3.0",
)

go_repository(
    name = "org_golang_x_time",
    build_file_proto_mode = "disable_global",
    commit = "fbb02b2291d2",
    importpath = "golang.org/x/time",
)

go_repository(
    name = "org_uber_go_atomic",
    build_file_proto_mode = "disable_global",
    importpath = "go.uber.org/atomic",
    tag = "v1.3.2",
)

go_repository(
    name = "org_uber_go_multierr",
    build_file_proto_mode = "disable_global",
    importpath = "go.uber.org/multierr",
    tag = "v1.1.0",
)

go_repository(
    name = "org_uber_go_zap",
    build_file_proto_mode = "disable_global",
    importpath = "go.uber.org/zap",
    tag = "v1.9.1",
)

go_repository(
    name = "com_github_akavel_rsrc",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/akavel/rsrc",
    tag = "v0.8.0",
)

go_repository(
    name = "com_github_daaku_go_zipexe",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/daaku/go.zipexe",
    tag = "v1.0.0",
)

go_repository(
    name = "com_github_geertjohan_go_incremental",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/GeertJohan/go.incremental",
    tag = "v1.0.0",
)

go_repository(
    name = "com_github_geertjohan_go_rice",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/GeertJohan/go.rice",
    tag = "v1.0.0",
)

go_repository(
    name = "com_github_jessevdk_go_flags",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/jessevdk/go-flags",
    tag = "v1.4.0",
)

go_repository(
    name = "com_github_nkovacs_streamquote",
    build_file_proto_mode = "disable_global",
    commit = "49af9bddb229",
    importpath = "github.com/nkovacs/streamquote",
)

go_repository(
    name = "com_github_valyala_bytebufferpool",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/valyala/bytebufferpool",
    tag = "v1.0.0",
)

go_repository(
    name = "com_github_valyala_fasttemplate",
    build_file_proto_mode = "disable_global",
    importpath = "github.com/valyala/fasttemplate",
    tag = "v1.0.1",
)
