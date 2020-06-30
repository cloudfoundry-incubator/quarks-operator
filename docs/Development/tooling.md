---
title: "Nice tools to use"
linkTitle: "Tooling"
weight: 4
description: >
  Tools to simplify your development workflow
---

The following is a list of tools with their respective main features that can help you
to simplify your development work when dealing with [quarks-operator](https://github.com/cloudfoundry-incubator/quarks-operator) and [kubecf](https://github.com/SUSE/kubecf)

### [k9s](https://github.com/derailed/k9s)

It provides an easy way to navigate through your k8s resources, while watching lively
to changes on them. Main features that can be helpful for containerized CF are:

* inmediate access to resources YAMLs definition

* inmediate access to services endpoints

* inmediate access to pods/container logs

* sort resources(e.g. pods) by cpu or memory consumption

* inmediate access to a container secure shell

### [havener](https://github.com/homeport/havener)

A tool-kit with different features around k8s and CloudFoundry

* `top`, to get an overview on the cpu/memory/load of the cluster, per ns and pods.

* `logs`, to download all logs from all pods into your local system

* `pod-exec`, to open a shell into containers. This can execute cmds in different containers
simultaneously.

* `node-exec`, to open a shell into nodes. This can execute cmds in different containers
simultaneously.

### [stern](https://github.com/wercker/stern)

Allows you to tail multiple pods on k8s and multiple containers within the pod.

### [kube dashboard](https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/)

A more user friendly to navigate your k8s cluster resources.