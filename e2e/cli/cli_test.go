package cli_test

import (
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("CLI", func() {
	act := func(arg ...string) (session *gexec.Session, err error) {
		cmd := exec.Command(cliPath, arg...)
		session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		return
	}

	Describe("help", func() {
		It("should show the help for server", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Usage:`))
		})

		It("should show all available options for server", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Flags:
      --apply-crd                                \(APPLY_CRD\) If true, apply CRDs on start \(default true\)
      --bosh-dns-docker-image string             \(BOSH_DNS_DOCKER_IMAGE\) The docker image used for emulating bosh DNS \(a CoreDNS image\) \(default "coredns/coredns:\d+.\d+.\d+"\)
  -n, --cf-operator-namespace string             \(CF_OPERATOR_NAMESPACE\) The operator namespace \(default "default"\)
      --cluster-domain string                    \(CLUSTER_DOMAIN\) The Kubernetes cluster domain \(default "cluster.local"\)
      --ctx-timeout int                          \(CTX_TIMEOUT\) context timeout for each k8s API request in seconds \(default 30\)
  -o, --docker-image-org string                  \(DOCKER_IMAGE_ORG\) Dockerhub organization that provides the operator docker image \(default "cfcontainerization"\)
      --docker-image-pull-policy string          \(DOCKER_IMAGE_PULL_POLICY\) Image pull policy \(default "IfNotPresent"\)
  -r, --docker-image-repository string           \(DOCKER_IMAGE_REPOSITORY\) Dockerhub repository that provides the operator docker image \(default "cf-operator"\)
  -t, --docker-image-tag string                  \(DOCKER_IMAGE_TAG\) Tag of the operator docker image \(default "\d+.\d+.\d+"\)
  -h, --help                                     help for cf-operator
  -c, --kubeconfig string                        \(KUBECONFIG\) Path to a kubeconfig, not required in-cluster
  -l, --log-level string                         \(LOG_LEVEL\) Only print log messages from this level onward \(default "debug"\)
      --max-boshdeployment-workers int           \(MAX_BOSHDEPLOYMENT_WORKERS\) Maximum number of workers concurrently running BOSHDeployment controller \(default 1\)
      --max-quarks-secret-workers int            \(MAX_QUARKS_SECRET_WORKERS\) Maximum number of workers concurrently running QuarksSecret controller \(default 5\)
      --max-quarks-statefulset-workers int       \(MAX_QUARKS_STATEFULSET_WORKERS\) Maximum number of workers concurrently running QuarksStatefulSet controller \(default 1\)
  -w, --operator-webhook-service-host string     \(CF_OPERATOR_WEBHOOK_SERVICE_HOST\) Hostname/IP under which the webhook server can be reached from the cluster
  -p, --operator-webhook-service-port string     \(CF_OPERATOR_WEBHOOK_SERVICE_PORT\) Port the webhook server listens on \(default "2999"\)
  -x, --operator-webhook-use-service-reference   \(CF_OPERATOR_WEBHOOK_USE_SERVICE_REFERENCE\) If true the webhook service is targeted using a service reference instead of a URL
      --watch-namespace string                   \(WATCH_NAMESPACE\) Namespace to watch for BOSH deployments`))
		})

		It("shows all available commands", func() {
			session, err := act("help")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Available Commands:
  help     \s* Help about any command
  util     \s* Calls a utility subcommand
  version  \s* Print the version number

`))
		})
	})

	Describe("default", func() {
		It("should start the server", func() {
			session, err := act()
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Err).Should(Say(`Starting cf-operator \d+\.\d+\.\d+ with namespace`))
			Eventually(session.Err).ShouldNot(Say(`Applying CRD...`))
		})

		Context("when specifying namespace", func() {
			Context("via environment variables", func() {
				BeforeEach(func() {
					os.Setenv("CF_OPERATOR_NAMESPACE", "env-test")
				})

				AfterEach(func() {
					os.Setenv("CF_OPERATOR_NAMESPACE", "")
				})

				It("should start for namespace", func() {
					session, err := act()
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Starting cf-operator \d+\.\d+\.\d+ with namespace env-test`))
				})
			})

			Context("via using switches", func() {
				It("should start for namespace", func() {
					session, err := act("--cf-operator-namespace", "switch-test")
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Starting cf-operator \d+\.\d+\.\d+ with namespace switch-test`))
				})
			})
		})

		Context("when enabling apply-crd", func() {
			Context("via environment variables", func() {
				BeforeEach(func() {
					os.Setenv("APPLY_CRD", "true")
				})

				AfterEach(func() {
					os.Setenv("APPLY_CRD", "")
				})

				It("should apply CRDs", func() {
					session, err := act()
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Applying CRDs...`))
				})
			})

			Context("via using switches", func() {
				It("should apply CRDs", func() {
					session, err := act("--apply-crd")
					Expect(err).ToNot(HaveOccurred())
					Eventually(session.Err).Should(Say(`Applying CRDs...`))
				})
			})
		})
	})

	Describe("version", func() {
		It("should show a semantic version number", func() {
			session, err := act("version")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`CF-Operator Version: \d+.\d+.\d+`))
		})
	})

	Describe("util", func() {
		It("should show util-wide flags incl. ENV binding", func() {
			session, err := act("util")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Flags:
  -h, --help   help for util`))
		})
	})

	Describe("variable-interpolation", func() {
		It("should list its flags incl. ENV binding", func() {
			session, err := act("util", "variable-interpolation", "-h")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Flags:
  -m, --bosh-manifest-path string   \(BOSH_MANIFEST_PATH\) path to the bosh manifest file
  -h, --help                        help for variable-interpolation
      --output-file-path string     \(OUTPUT_FILE_PATH\) Path of the file to which json output is written.
  -v, --variables-dir string        \(VARIABLES_DIR\) path to the variables dir`))
		})

		It("accepts the bosh-manifest-path as a parameter", func() {
			session, err := act("util", "variable-interpolation", "-m", "foo.txt", "--output-file-path", "./output.json")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Err).Should(Say("variable-interpolation command failed. bosh-manifest-path file doesn't exist : foo.txt"))
		})

		Context("using env variables for parameters", func() {
			BeforeEach(func() {
				os.Setenv("BOSH_MANIFEST_PATH", "bar.txt")
			})

			AfterEach(func() {
				os.Setenv("BOSH_MANIFEST_PATH", "")
			})

			It("accepts the bosh-manifest-path as an environment variable", func() {
				session, err := act("util", "variable-interpolation", "--output-file-path", "./output.json")
				Expect(err).ToNot(HaveOccurred())
				Eventually(session.Err).Should(Say("variable-interpolation command failed. bosh-manifest-path file doesn't exist : bar.txt"))
			})
		})
	})

	Describe("instance-group", func() {
		It("lists its flags incl. ENV binding", func() {
			session, err := act("util", "instance-group", "-h")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Flags:
  -b, --base-dir string              \(BASE_DIR\) a path to the base directory
  -m, --bosh-manifest-path string    \(BOSH_MANIFEST_PATH\) path to the bosh manifest file
  -h, --help                         help for instance-group
      --initial-rollout              \(INITIAL_ROLLOUT\) Initial rollout of bosh deployment. \(default true\)
  -g, --instance-group-name string   \(INSTANCE_GROUP_NAME\) name of the instance group for data gathering
      --output-file-path string      \(OUTPUT_FILE_PATH\) Path of the file to which json output is written.`))
		})

		It("accepts the bosh-manifest-path as a parameter", func() {
			session, err := act("util", "instance-group", "--base-dir=.", "-m", "foo.txt", "-g", "log-api", "--output-file-path", "./output.json")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Err).Should(Say("open foo.txt: no such file or directory"))
		})

		Context("using env variables for parameters", func() {
			BeforeEach(func() {
				os.Setenv("BOSH_MANIFEST_PATH", "bar.txt")
			})

			AfterEach(func() {
				os.Setenv("BOSH_MANIFEST_PATH", "")
			})

			It("accepts the bosh-manifest-path as an environment variable", func() {
				session, err := act("util", "instance-group", "--base-dir=.", "-g", "log-api", "--output-file-path", "./output.json")
				Expect(err).ToNot(HaveOccurred())
				Eventually(session.Err).Should(Say("open bar.txt: no such file or directory"))
			})
		})
	})

	Describe("bpm-configs", func() {
		It("lists its flags incl. ENV binding", func() {
			session, err := act("util", "bpm-configs", "-h")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Flags:
  -b, --base-dir string              \(BASE_DIR\) a path to the base directory
  -m, --bosh-manifest-path string    \(BOSH_MANIFEST_PATH\) path to the bosh manifest file
  -h, --help                         help for bpm-configs
      --initial-rollout              \(INITIAL_ROLLOUT\) Initial rollout of bosh deployment. \(default true\)
  -g, --instance-group-name string   \(INSTANCE_GROUP_NAME\) name of the instance group for data gathering
      --output-file-path string      \(OUTPUT_FILE_PATH\) Path of the file to which json output is written.`))
		})

		It("accepts the bosh-manifest-path as a parameter", func() {
			session, err := act("util", "bpm-configs", "--base-dir=.", "-m", "foo.txt", "-g", "log-api", "--output-file-path", "./output.json")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Err).Should(Say("open foo.txt: no such file or directory"))
		})

		Context("using env variables for parameters", func() {
			BeforeEach(func() {
				os.Setenv("BOSH_MANIFEST_PATH", "bar.txt")
			})

			AfterEach(func() {
				os.Setenv("BOSH_MANIFEST_PATH", "")
			})

			It("accepts the bosh-manifest-path as an environment variable", func() {
				session, err := act("util", "bpm-configs", "--base-dir=.", "-g", "log-api", "--output-file-path", "./output.json")
				Expect(err).ToNot(HaveOccurred())
				Eventually(session.Err).Should(Say("open bar.txt: no such file or directory"))
			})
		})
	})

	Describe("template-render", func() {
		It("lists its flags incl. ENV binding", func() {
			session, err := act("util", "template-render", "-h")
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Out).Should(Say(`Flags:
      --az-index int                 \(AZ_INDEX\) az index \(default -1\)
  -m, --bosh-manifest-path string    \(BOSH_MANIFEST_PATH\) path to the bosh manifest file
  -h, --help                         help for template-render
      --initial-rollout              \(INITIAL_ROLLOUT\) Initial rollout of bosh deployment. \(default true\)
  -g, --instance-group-name string   \(INSTANCE_GROUP_NAME\) name of the instance group for data gathering
  -j, --jobs-dir string              \(JOBS_DIR\) path to the jobs dir.
  -d, --output-dir string            \(OUTPUT_DIR\) path to output dir. \(default "/var/vcap/jobs"\)
      --pod-ip string                \(POD_IP\) pod IP
      --pod-ordinal int              \(POD_ORDINAL\) pod ordinal \(default -1\)
      --replicas int                 \(REPLICAS\) number of replicas \(default -1\)
      --spec-index int               \(SPEC_INDEX\) index of the instance spec \(default -1\)
`))
		})

		It("accepts the bosh-manifest-path as a parameter", func() {
			session, err := act(
				"util", "template-render",
				"--az-index=1",
				"--replicas=1",
				"--pod-ordinal=1",
				"-m", "foo.txt",
				"-g", "log-api",
				"--pod-ip", "127.0.0.1",
			)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session.Err).Should(Say("open foo.txt: no such file or directory"))
		})

		Context("using env variables for parameters", func() {
			BeforeEach(func() {
				os.Setenv("BOSH_MANIFEST_PATH", "bar.txt")
			})

			AfterEach(func() {
				os.Setenv("BOSH_MANIFEST_PATH", "")
			})

			It("accepts the bosh-manifest-path as an environment variable", func() {
				session, err := act(
					"util", "template-render",
					"--az-index=1",
					"--replicas=1",
					"--pod-ordinal=1",
					"-g", "log-api",
					"--pod-ip", "127.0.0.1",
				)
				Expect(err).ToNot(HaveOccurred())
				Eventually(session.Err).Should(Say("open bar.txt: no such file or directory"))
			})
		})
	})
})
