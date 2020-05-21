package kindest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Jeffail/tunny"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newCLI(t *testing.T) client.APIClient {
	cli, err := client.NewEnvClient()
	require.NoError(t, err)
	return cli
}

func createBasicTestProject(t *testing.T, path string) string {
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join(path, name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	dockerfile := `FROM alpine:latest
CMD ["sh", "-c", "echo \"Hello, world\""]`
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(rootPath, "Dockerfile"),
		[]byte(dockerfile),
		0644,
	))
	specPath := filepath.Join(rootPath, "kindest.yaml")
	spec := fmt.Sprintf(`build:
  name: test/%s
  docker: {}
`, name)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	return specPath
}

func TestBuildBasic(t *testing.T) {
	specPath := createBasicTestProject(t, "tmp")
	defer os.RemoveAll(filepath.Dir(specPath))
	require.NoError(t, Build(
		&BuildOptions{
			File: specPath,
		},
		newCLI(t),
	))
}

func TestBuildCustomDockerfilePath(t *testing.T) {
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join("tmp", name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	defer os.RemoveAll(rootPath)
	subdir := filepath.Join(rootPath, "subdir")
	require.NoError(t, os.MkdirAll(subdir, 0766))
	dockerfile := `FROM alpine:latest
CMD ["sh", "-c", "echo \"Hello, world\""]`
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(subdir, "Dockerfile"),
		[]byte(dockerfile),
		0644,
	))
	specPath := filepath.Join(rootPath, "kindest.yaml")
	spec := fmt.Sprintf(`build:
  name: test/%s
  docker:
    dockerfile: subdir/Dockerfile
`, name)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	require.NoError(t, Build(
		&BuildOptions{File: specPath},
		newCLI(t),
	))
}

func TestBuildErrMissingBuildArg(t *testing.T) {
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join("tmp", name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	defer os.RemoveAll(rootPath)
	dockerfile := `FROM alpine:latest
ARG HAS_BUILD_ARG
RUN if [ -z "$HAS_BUILD_ARG" ]; then exit 1; fi`
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(rootPath, "Dockerfile"),
		[]byte(dockerfile),
		0644,
	))
	specPath := filepath.Join(rootPath, "kindest.yaml")
	spec := fmt.Sprintf(`build:
  name: test/%s
  docker: {}
`, name)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	require.Error(t, Build(
		&BuildOptions{File: specPath},
		newCLI(t),
	))
}

func TestBuildArg(t *testing.T) {
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join("tmp", name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	defer os.RemoveAll(rootPath)
	dockerfile := `FROM alpine:latest
ARG HAS_BUILD_ARG
RUN if [ -z "$HAS_BUILD_ARG" ]; then exit 1; fi`
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(rootPath, "Dockerfile"),
		[]byte(dockerfile),
		0644,
	))
	specPath := filepath.Join(rootPath, "kindest.yaml")
	spec := fmt.Sprintf(`build:
  docker:
    name: test/%s
	buildArgs:
	  - name: HAS_BUILD_ARG
	    value: "1"
`, name)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	require.Error(t, Build(
		&BuildOptions{File: specPath},
		newCLI(t),
	))
}

func TestBuildContextPath(t *testing.T) {
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join("tmp", name)
	subdir := filepath.Join(rootPath, "subdir")
	otherdir := filepath.Join(rootPath, "other")
	require.NoError(t, os.MkdirAll(subdir, 0766))
	require.NoError(t, os.MkdirAll(otherdir, 0766))
	defer os.RemoveAll(rootPath)
	dockerfile := `FROM alpine:latest
CMD ["sh", "-c", "echo \"Hello, world\""]`
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(otherdir, "Dockerfile"),
		[]byte(dockerfile),
		0644,
	))
	specPath := filepath.Join(otherdir, "kindest.yaml")
	spec := fmt.Sprintf(`build:
  name: test/%s
  docker:
    dockerfile: "../other/Dockerfile"
    context: ".."
`, name)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	require.NoError(t, Build(
		&BuildOptions{File: specPath},
		newCLI(t),
	))
}

func TestBuildDependency(t *testing.T) {
	depName := "test-" + uuid.New().String()[:8]
	name := "test-" + uuid.New().String()[:8]
	log.Info("Building dep test", zap.String("depName", depName), zap.String("name", name))
	rootPath := filepath.Join("tmp", name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	// Use the dependency as a base image
	dockerfile := fmt.Sprintf(`FROM test/%s:latest
CMD ["sh", "-c", "echo \"Hello again, world\""]`, depName)
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(rootPath, "Dockerfile"),
		[]byte(dockerfile),
		0644,
	))
	specPath := filepath.Join(rootPath, "kindest.yaml")
	spec := fmt.Sprintf(`dependencies:
  - dep
build:
  name: test/%s
  docker: {}
`, name)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	depDockerfile := `FROM alpine:latest
CMD ["sh", "-c", "echo \"Hello, world\""]`
	depPath := filepath.Join(rootPath, "dep")
	require.NoError(t, os.MkdirAll(depPath, 0766))
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(depPath, "Dockerfile"),
		[]byte(depDockerfile),
		0644,
	))
	depSpec := fmt.Sprintf(`build:
  name: test/%s
  docker: {}
`, depName)
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(depPath, "kindest.yaml"),
		[]byte(depSpec),
		0644,
	))
	require.NoError(t, Build(
		&BuildOptions{
			File: specPath,
		},
		newCLI(t),
	))
}

func TestBuildCache(t *testing.T) {
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join("tmp", name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	defer os.RemoveAll(rootPath)
	buildArg := uuid.New().String()
	dockerfile := `FROM alpine:latest
ARG BUILD_ARG
RUN echo '#!/bin/sh' >> /script
RUN echo 'echo \"${BUILD_ARG}\"' >> /script
RUN chmod +x /script
RUN /script
CMD ["cat", "/script"]`
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(rootPath, "Dockerfile"),
		[]byte(dockerfile),
		0644,
	))
	specPath := filepath.Join(rootPath, "kindest.yaml")
	spec := fmt.Sprintf(`build:
  name: test/%s
  docker:
    buildArgs:
    - name: BUILD_ARG
      value: "%s"
`, name, buildArg)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	cli := newCLI(t)
	done := make(chan error, 1)
	var pool *tunny.Pool
	pool = tunny.NewFunc(runtime.NumCPU(), func(payload interface{}) interface{} {
		options := payload.(*BuildOptions)
		return BuildEx(options, cli, pool, func(r io.ReadCloser) {
			defer close(done)
			rd := bufio.NewReader(r)
			isUsingCache := false
			for {
				message, err := rd.ReadString('\n')
				if err != nil {
					require.True(t, err == io.EOF)
					break
				}
				var msg struct {
					Stream string `json:"stream"`
				}
				require.NoError(t, json.Unmarshal([]byte(message), &msg))
				log.Info("Docker", zap.String("message", msg.Stream))
				if strings.Contains(msg.Stream, "Using cache") {
					isUsingCache = true
				}
			}
			if !isUsingCache {
				done <- fmt.Errorf("docker build cache was not used")
			} else {
				done <- nil
			}
		})
	})
	defer pool.Close()
	err, _ := pool.Process(&BuildOptions{File: specPath}).(error)
	require.NoError(t, err)
	require.NoError(t, <-done)
}
