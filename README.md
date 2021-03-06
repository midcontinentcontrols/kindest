**Use one of the tags (i.e. `git checkout 0.0.3`) if you desire a stable release. Master breaks frequently.**

# Kindest DevOps for Monorepos

This is a toolchain built on top of [kind](https://github.com/kubernetes-sigs/kind) that aims to reduce the complexity associated with using it as a tool for microservice development. It was born out of necessity to reduce increasing execution times and maintenance overhead of bash scripts that accomplished more or less the same thing.

Currently, kindest only runs locally using the Docker daemon. It is already a suitable replacement for `docker build`. There are plans to support on-cluster building with [kaniko](https://github.com/GoogleContainerTools/kaniko).

At its core, the `kindest.yaml` file defines how images are built and tested either locally or on Kubernetes. The build process is fully parallelized and utilizes caching, so as to bypass redundant work for each submodule when no files have changed.

The testing features are stable but currently lack documentation. To run tests on a kind cluster using the [local registry](https://kind.sigs.k8s.io/docs/user/local-registry/), use the command `kindest test --kind my-cluster --repository localhost:5000`

### kindest.yaml
```yaml
# Relative paths to any dependent modules. Building or
# testing this module will do the same for all deps.
# These are built/tested concurrently, so it can be more
# generally exploited to parallelize build jobs.
dependencies:
  - my-submodule # ./my-submodule/kindest.yaml

build:
  # Name of the Docker image to build. The tag is chosen
  # by kindest, so only specify repo and image.
  name: midcontinentcontrols/example-image

  #dockerfile: ./Dockerfile

  # Docker build context, relative to kindest.yaml
  #context: .

  # https://docs.docker.com/engine/reference/commandline/build/
  #buildArgs:
  #  - name: ARG_NAME
  #    value: ARG_VALUE

  # Default builder will choose docker for building locally and
  # kaniko for on-cluster. For building architectures not supported
  # by kaniko (e.g. arm32) specifying `docker` here will mount the
  # node's docker.sock into the build pod and use that instead.
  # Note: if a builder preference of `docker` is enforced, invoking
  # `kindest build --builder kaniko` will raise an error.
  #builder: "" # "docker", "kaniko"

  # Only schedule the builder pod on nodes with these labels.
  # In multiarch environments, this is useful for ensuring images
  # are built only on nodes with compatible architecture.
  #nodeSelector:
  # arm32: yes    # Schedule on nodes labeled with `kubectl label node my-node arm32=yes`

  # Commands to run locally before building
  #before: []

  # Commands to run locally after building/pushing
  #after: []

test:
  # Tests have a `build` section mirroring the module's.
  # Typically, this image will contain source code for all
  # the repo's dependencies and potentially be multiple gb
  # in size. 
  build:
    name: midcontinentcontrols/example-test
    dockerfile: test/Dockerfile
  
  # We can specify a minimal environment for running the tests
  env:
    # Run the test image locally with Docker. This is useful
    # for incorporating unit tests into your kindest.yaml
    #docker: {}

    # Run the test in a Kubernetes environment. The cluster
    # may be an actual k8s cluster or a KIND cluster running
    # either locally or inside a pod. 
    kubernetes:
      # List of relative paths to manifests that should be
      # applied before running tests.
      resources:
        - path/to/my_custom_resource_definitions.yaml

      # These charts will be installed/upgraded before the
      # tests run.
      charts:
        - releaseName: kindest
          namespace: kindest
          path: path/to/chart # ./path/to/chart/Chart.yaml
          values: {}

    # List of environment variables that will be passed to
    # the test container.
    variables:
      - name: EXAMPLE_DEPENDENCY_URI
        value: http://example-dependency-microservice.default.svc.cluster.local:5000
```

## Test Environments
The minimal environment required by the tests is configured under the `test.env:` section of `kindest.yaml`. 

### Docker
For tests that do not *require* Kubernetes (e.g. unit tests), there is Docker executor that runs the test image in isolation: 

```yaml
# kindest.yaml
test:
  build:
    # Name of the test image. This image is not pushed unless
    # necessary, so you can use whatever you want.
    name: midcontinentcontrols/example-test-image

    # Custom dockerfile path typically has to be assigned
    # for tests, as it defaults to ./Dockerfile. The build
    # context will be the root and includes ./test/Dockerfile
    dockerfile: test/Dockerfile

  env:
    # Run tests in a simple Docker container. Note: if no
    # additional settings are required, the empty object
    # `{}` is used to make the field non-nil
    docker: {}

    # Environment variables passed to the test container
    #variables:
    #- name: FOO
    #  value: BAR
```

Note that tests using the Docker environment can still run as pods on Kubernetes, but they will not have access to the cluster.

### Kubernetes
Tests requiring a Kubernetes cluster can indicate the minimal requirements by defining the `test.env.kubernetes:` object. 

```yaml
# kindest.yaml
test:
  ...
  env:
    # Run these tests on kubernetes
    kubernetes:
      # Paths to resources that will be applied every time
      # before running tests. Use this to load CRDs and
      # secrets. Remember to add your secrets to .gitignore
      # so they are not checked into source control.
      resources:
        - crds/my-example-crd.yaml
        - test/fixtures/secrets/my-secret.yaml

      # These charts will be installed/upgraded before the
      # tests run.
      charts:
        - releaseName: kindest
          path: ./charts/kindest # ./charts/kindest/Chart.yaml
          values: {}
```

The test container will now run as a pod on Kubernetes with cluster admin privileges. Any setup necessary for your tests, such as applying manifests or syncing helm charts, should go in `test.env.kubernetes`. If you require additional features to prepare your test environment, please [open an issue](https://github.com/midcontinentcontrols/kindest/issues).

## Docker Desktop Resource Limits
**The default resource limits for Docker Desktop appear insufficient to run the tests.** If this occurs, you will encounter [kind#1437](https://github.com/kubernetes-sigs/kind/issues/1437#issuecomment-602975739). Configure Docker with 4gb of both memory and swap just to be safe:

![Example Docker Desktop Settings](docs/images/docker-resources.png)

## Security
Running kind in a Kubernetes pod poses security risks worthy of operator attention. The Docker daemon of the node, running as root, is exposed to the test cluster. This is considered acceptable when running trusted code on dedicated hardware, which is the target use case of kindest. Open source developers in particular should consider the risks of using kindest with their community CI, and take appropriate mitigating measures.

Even when not using kind, test pods always run with cluster admin permissions. This may change in the future, but for now the goal is to keep things simple.

## Contributing
Please open an issue or [send an email](mailto:thavlik@midcontinentcontrols.com) if you would like to contribute or offer feedback. 

### Running the Tests
The unit tests aim to be comprehensive. To run them with full console output:
```
git clone git@github.com:midcontinentcontrols/kindest.git
cd kindest/pkg/kindest
go test -v -timeout 4h
```
Image pulling and building is part of the tests, so the `-timeout` flag is necessary. `.vscode/settings.json` is intentionally part of this repository in order for VS Code's `Go: Toggle Test Coverage in Current Package` to work correctly.

## License
Copyright (c) Mid Continent Controls, Inc. 2020

Released under MIT and Apache dual licenses. Unencumbered commercial use is permitted. See LICENSE-Apache and LICENSE-MIT files for more information.
