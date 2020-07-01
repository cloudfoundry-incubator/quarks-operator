#!/bin/bash

set -e

NS="$1"

echo "Collecting environment information for namespace ${NS}"

function get_resources() {
  kubectl get "$1" --namespace "${NS}" --output=jsonpath='{.items[*].metadata.name}'
}

function describe_resource() {
  kubectl describe "$1" "$2" --namespace "${NS}" > "$3"
}

function get_resource() {
  kubectl get "$1" "$2" --output=yaml --namespace "${NS}" > "$3"
}

function get_pod_phase() {
  kubectl get pod "${POD}" --namespace "${NS}" --output=jsonpath='{.status.phase}'
}

function get_containers_of_pod() {
  kubectl get pods "${POD}" --namespace "${NS}" --output=jsonpath='{.spec.containers[*].name}'
}

function get_init_containers_of_pod() {
  kubectl get pods "${POD}" --namespace "${NS}" --output=jsonpath='{.spec.initContainers[*].name}'
}

function retrieve_container_kube_logs() {
  printf " Kube logs"
  kubectl logs "${POD}" --namespace "${NS}" --container "${CONTAINER}"            > "${CONTAINER_DIR}/kube.log"
  kubectl logs "${POD}" --namespace "${NS}" --container "${CONTAINER}" --previous > "${CONTAINER_DIR}/kube-previous.log"
}

function retrieve_init_container_kube_logs() {
  printf " Kube logs"
  kubectl logs "${POD}" --namespace "${NS}" --container "${INITCONTAINER}"            > "${INIT_CONTAINER_DIR}/kube.log"
}

function get_all_resources() {
  printf "Resources ...\n"
  kubectl get all --namespace "${NS}" --output=yaml > "${NAMESPACE_DIR}/resources.yaml"
}

function get_all_events() {
  printf "Events ...\n"
  kubectl get events --namespace "${NS}" --output=yaml > "${NAMESPACE_DIR}/events.yaml"
}

OUTPUT_BASE="${CF_OPERATOR_TESTING_TMP:-/tmp}"
NAMESPACE_DIR="/${OUTPUT_BASE}/env_dumps/${NS}"
if [ -e "$NAMESPACE_DIR" ]; then
  i=1
  while [ -e "/${OUTPUT_BASE}/env_dumps/${NS}-$i" ]; do
    let i++
  done
  NAMESPACE_DIR="/${OUTPUT_BASE}/env_dumps/${NS}-$i"
fi

printf "Output directory: $(basename $NAMESPACE_DIR)...\n"
SECRETS_DIR="${NAMESPACE_DIR}/secrets"
CONFIGMAPS_DIR="${NAMESPACE_DIR}/configmaps"
mkdir -p "${OUTPUT_BASE}/env_dumps"
mkdir -p "${SECRETS_DIR}"
mkdir -p "${CONFIGMAPS_DIR}"

# Iterate over configmaps
CONFIGMAPS=($(get_resources "configmaps" ))
for CM in "${CONFIGMAPS[@]}"; do
  printf "Configmap \e[0;32m$CM\e[0m\n"
  get_resource configmap "$CM" "${CONFIGMAPS_DIR}/${CM}.yaml"
done

# Iterate over secrets
SECRETS=($(get_resources "secrets"))
for SECRET in "${SECRETS[@]}"; do
  if grep -qv "token" <<< "$SECRET"; then
    printf "Secret \e[0;32m$SECRET\e[0m\n"
    get_resource secret "$SECRET" "${SECRETS_DIR}/${SECRET}.yaml"
  fi
done

# Iterate over jobs, Quarks*
for i in jobs qsts qjobs qsecs sts; do
  RESOURCES=($(get_resources "$i"))
  for RESOURCE in "${RESOURCES[@]}"; do
    printf "$i \e[0;32m$RESOURCE\e[0m\n"

    RESOURCE_DIR="${NAMESPACE_DIR}/$i/${RESOURCE}"
    mkdir -p ${RESOURCE_DIR}
    describe_resource "$i" "$RESOURCE" "${RESOURCE_DIR}/describe.txt"
  done
done

# Iterate over pods and their containers
PODS=($(get_resources "pods"))
for POD in "${PODS[@]}"; do
  POD_DIR="${NAMESPACE_DIR}/${POD}"
  PHASE="$(get_pod_phase)"

  printf "Pod \e[0;32m$POD\e[0m = $PHASE\n"

  # Iterate over containers and dump logs.
  CONTAINERS=($(get_containers_of_pod))
  for CONTAINER in "${CONTAINERS[@]}"; do
    printf "  container - \e[0;32m${CONTAINER}\e[0m logs:"

    CONTAINER_DIR="${POD_DIR}/${CONTAINER}"
    mkdir -p "${CONTAINER_DIR}"
    retrieve_container_kube_logs 2> /dev/null || true

    printf "\n"
  done

  # Iterate over initContainers and dump logs.
  INITCONTAINERS=($(get_init_containers_of_pod))
  for INITCONTAINER in "${INITCONTAINERS[@]}"; do
    printf "  initContainer - \e[0;32m${INITCONTAINER}\e[0m logs:"

    INIT_CONTAINER_DIR="${POD_DIR}/init-containers/${INITCONTAINER}"
    mkdir -p "${INIT_CONTAINER_DIR}"
    retrieve_init_container_kube_logs 2> /dev/null || true

    printf "\n"
  done

  describe_resource pod "$POD" "${POD_DIR}/describe-pod.txt"
done

get_all_resources
get_all_events

find "$NAMESPACE_DIR" -type f -size 0 -delete
find "$NAMESPACE_DIR" -type d -empty -delete

printf "\e[0;32mDone\e[0m\n"
exit
