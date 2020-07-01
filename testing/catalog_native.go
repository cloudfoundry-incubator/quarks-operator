package testing

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
)

// NatsPod returns a Pod used to test native-to-bosh quarks-links
func (c *Catalog) NatsPod(deployName string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nats",
			Labels: map[string]string{
				bdv1.LabelDeploymentName: deployName,
				"app":                    "nats-app",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "nats",
					Image:           "docker.io/bitnami/nats:1.1.0",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"gnatsd"},
					Args:            []string{"-c", "/opt/bitnami/nats/gnatsd.conf"},
					Ports: []corev1.ContainerPort{
						{
							Name:          "client",
							ContainerPort: 4222,
						},
						{
							Name:          "cluster",
							ContainerPort: 6222,
						},
						{
							Name:          "monitoring",
							ContainerPort: 8222,
						},
					},
					LivenessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/",
								Port: intstr.FromString("monitoring"),
							},
						},
						FailureThreshold:    6,
						PeriodSeconds:       10,
						SuccessThreshold:    1,
						TimeoutSeconds:      5,
						InitialDelaySeconds: 30,
					},
					ReadinessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/",
								Port: intstr.FromString("monitoring"),
							},
						},
						FailureThreshold:    6,
						PeriodSeconds:       10,
						SuccessThreshold:    1,
						TimeoutSeconds:      5,
						InitialDelaySeconds: 5,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "config",
							MountPath: "/opt/bitnami/nats/gnatsd.conf",
							SubPath:   "gnatsd.conf",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "nats",
							},
						},
					},
				},
			},
		},
	}
}

// NatsConfigMap returns a ConfigMap used to test native-to-bosh quarks-links
func (c *Catalog) NatsConfigMap(deployName string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nats",
		},
		Data: map[string]string{
			"gnatsd.conf": `listen: 0.0.0.0:4222
http: 0.0.0.0:8222

# Authorization for client connections
authorization {
  user: nats_client
  password: r9fXAlY3gZ
  timeout:  1
}

# Logging options
debug: false
trace: false
logtime: false

# Pid file
pid_file: "/tmp/gnatsd.pid"

# Some system overides


# Clustering definition
cluster {
  listen: 0.0.0.0:6222

  # Authorization for cluster connections
  authorization {
	user: nats_cluster
	password: hK9awRcEYs
	timeout:  1
  }

  # Routes are actively solicited and connected to from this server.
  # Other servers can connect to us if they supply the correct credentials
  # in their routes definitions from above
  routes = [
	nats://nats_cluster:hK9awRcEYs@nats-headless:6222
  ]
}`,
		},
	}
}

// NatsService is used to test native-to-bosh quarks-links
func (c *Catalog) NatsService(deployName string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nats-headless",
			Labels: map[string]string{
				bdv1.LabelDeploymentName: deployName,
			},
			Annotations: map[string]string{
				bdv1.AnnotationLinkProviderName: "nats",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": "nats-app",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "client",
					Port:       4222,
					TargetPort: intstr.FromString("client"),
				},
				{
					Name:       "cluster",
					Port:       6222,
					TargetPort: intstr.FromString("cluster"),
				},
			},
		},
	}
}

// NatsServiceExternalName is used to test native-to-bosh quarks-links
func (c *Catalog) NatsServiceExternalName(deployName string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nats-headless",
			Labels: map[string]string{
				bdv1.LabelDeploymentName: deployName,
			},
			Annotations: map[string]string{
				bdv1.AnnotationLinkProviderName: "nats",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: "test.example.com",
		},
	}
}

// NatsServiceForEndpoint is used to test native-to-bosh quarks-links
func (c *Catalog) NatsServiceForEndpoint(deployName string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nats-ep",
			Labels: map[string]string{
				bdv1.LabelDeploymentName: deployName,
			},
			Annotations: map[string]string{
				bdv1.AnnotationLinkProviderName: "nats",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "client",
					Port:       4222,
					TargetPort: intstr.FromString("client"),
				},
				{
					Name:       "cluster",
					Port:       6222,
					TargetPort: intstr.FromString("cluster"),
				},
			},
		},
	}
}

// NatsEndpoints is used to test native-to-bosh quarks-links
func (c *Catalog) NatsEndpoints(deployName string) corev1.Endpoints {
	return corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nats-ep",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Ports:     []corev1.EndpointPort{{Port: 1234}},
				Addresses: []corev1.EndpointAddress{{IP: "192.168.0.1"}},
			},
		},
	}
}

// NatsSecret is used to test native-to-bosh quarks-links
func (c *Catalog) NatsSecret(deployName string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nats-sec",
			Labels: map[string]string{
				bdv1.LabelDeploymentName: deployName,
			},
			Annotations: map[string]string{
				bdv1.AnnotationLinkProvidesKey: `{"name":"nats","type":"nats"}`,
			},
		},
		StringData: map[string]string{
			"link": `---
nats.user: nats_client
nats.password: r9fXAlY3gZ
nats.port: "4222"`,
		},
	}
}

// NatsOtherSecret is used to test native-to-bosh quarks-links
func (c *Catalog) NatsOtherSecret(deployName string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nats-sec",
			Labels: map[string]string{
				bdv1.LabelDeploymentName: deployName,
			},
			Annotations: map[string]string{
				bdv1.AnnotationLinkProvidesKey: `{"name":"nats","type":"nats"}`,
			},
		},
		StringData: map[string]string{
			"link": `---
nats.user: nats_user
nats.password: abcdefg123
nats.port: "4222"`,
		},
	}
}
