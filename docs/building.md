# Build

The following steps layout the process of generating a `cf-operator` controller and how to install this in your Kubernetes cluster. This is probably a good approach, when developing, as a way to inmediately test your changes.

## Build it from source

Follow this steps to build a proper docker image and generate a deployable helm chart:

1. Checkout the latest stable release / or run it from develop branch

    ```bash
    git checkout v0.3.0
    ```

2. Build the cf-operator binary, this will be embedded later on the docker image

    ```bash
    bin/build
    ```

3. Build the docker image

    When running in minikube, please run: `eval $(minikube docker-env)`, to build the image
    directly on minikube docker.

    ```bash
    bin/build-image
    ```

    _**Note**_: This will automatically generate a docker image tag based on your current commit, tag and SHA.

4. Generated helm charts with a proper docker image tag, org and repository

    ```bash
    bin/build-helm
    ```

    _**Note**_: This will generate a new directory under the base dir, named `helm/`

5. Install the helm chart(apply Kubernetes Custom Resources)

    ```bash
    helm install cf-operator-test helm/cf-operator
    ```

    _**Note**_: The cf-operator will be available under the namespace set in the context, usually `default`, running as a pod.

## Notes

### Local Development with Minikube and Havener

Make sure you have [havener](https://github.com/homeport/havener) install.

```bash
havener deploy --config dev-env-havener.yaml
```
