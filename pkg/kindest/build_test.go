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
	"sync/atomic"
	"testing"

	"github.com/Jeffail/tunny"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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
	dockerfile := `FROM alpine:3.11.6
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

func TestBuildErrDependencyBuildFailure(t *testing.T) {
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join("tmp", name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	defer os.RemoveAll(rootPath)
	dockerfile := `FROM alpine:3.11.6
CMD ["sh", "-c", "echo \"Hello, world\""]`
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(rootPath, "Dockerfile"),
		[]byte(dockerfile),
		0644,
	))
	specPath := filepath.Join(rootPath, "kindest.yaml")
	spec := fmt.Sprintf(`dependencies: ["dep"]
build:
  name: test/%s
  docker: {}`, name)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	depPath := filepath.Join(rootPath, "dep")
	require.NoError(t, os.MkdirAll(depPath, 0766))
	depSpec := fmt.Sprintf(`
build:
  name: test/%s-dep
  docker: {}`, name)
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(depPath, "kindest.yaml"),
		[]byte(depSpec),
		0644,
	))
	depDockerfile := `FROM alpine:3.11.6
RUN exit 1`
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(depPath, "Dockerfile"),
		[]byte(depDockerfile),
		0644,
	))
	err := Build(
		&BuildOptions{
			File: specPath,
		},
		newCLI(t),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dependency 'dep': The command '/bin/sh -c exit 1' returned a non-zero code: 1")
}

func TestBuildErrMissingDependencySpec(t *testing.T) {
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join("tmp", name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	defer os.RemoveAll(rootPath)
	dockerfile := `FROM alpine:3.11.6
CMD ["sh", "-c", "echo \"Hello, world\""]`
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(rootPath, "Dockerfile"),
		[]byte(dockerfile),
		0644,
	))
	specPath := filepath.Join(rootPath, "kindest.yaml")
	spec := fmt.Sprintf(`dependencies: ["dep"]
build:
  name: test/%s
  docker: {}`, name)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	err := Build(
		&BuildOptions{
			File: specPath,
		},
		newCLI(t),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dependency 0: missing kindest.yaml")
}

func TestBuildErrMissingDockerfile(t *testing.T) {
	specPath := createBasicTestProject(t, "tmp")
	rootPath := filepath.Dir(specPath)
	defer os.RemoveAll(rootPath)
	require.NoError(t, os.Remove(filepath.Join(rootPath, "Dockerfile")))
	err := Build(
		&BuildOptions{
			File: specPath,
		},
		newCLI(t),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing Dockerfile")
}

func TestBuildDockerfile(t *testing.T) {
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join("tmp", name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	defer os.RemoveAll(rootPath)
	subdir := filepath.Join(rootPath, "subdir")
	require.NoError(t, os.MkdirAll(subdir, 0766))
	dockerfile := `FROM alpine:3.11.6
CMD ["sh", "-c", "echo \"Hello, world\""]`
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(subdir, "Dockerfile"),
		[]byte(dockerfile),
		0644,
	))
	specPath := filepath.Join(rootPath, "kindest.yaml")
	spec := fmt.Sprintf(`build:
  name: test/%s
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

func TestBuildTarget(t *testing.T) {
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join("tmp", name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	defer os.RemoveAll(rootPath)
	subdir := filepath.Join(rootPath, "subdir")
	require.NoError(t, os.MkdirAll(subdir, 0766))
	dockerfile := `FROM alpine:3.11.6 AS builder
RUN apk add --no-cache bash
RUN echo "foobarbaz" >> /foobarbaz
FROM alpine:3.11.6
COPY --from=builder /foobarbaz /bal
CMD ["sh", "-c", "echo \"Hello, world\""]`
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(subdir, "Dockerfile"),
		[]byte(dockerfile),
		0644,
	))
	specPath := filepath.Join(rootPath, "kindest.yaml")
	spec := fmt.Sprintf(`build:
  name: test/%s
  dockerfile: subdir/Dockerfile
test:
  - name: "basic"
    env:
      docker: {}
    build:
      name: test/%s-builder
      dockerfile: subdir/Dockerfile
      target: builder
      command:
        - bash
        - -c
        - if [ -z "$(ls / | grep foobarbaz)" ]; then
            exit 3;
          fi;
          if [ -n "$(ls / | grep bal)" ]; then
            exit 3;
          fi;
          echo "This script is executing in the correct layer."
`, name, name)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	require.NoError(t, Test(
		&TestOptions{
			File:       specPath,
			NoRegistry: true,
		},
	))
}

func TestBuildErrMissingName(t *testing.T) {
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join("tmp", name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	defer os.RemoveAll(rootPath)
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(rootPath, "Dockerfile"),
		[]byte("FROM alpine:3.11.6"),
		0644,
	))
	specPath := filepath.Join(rootPath, "kindest.yaml")
	spec := fmt.Sprintf(`build: {}`)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	require.Equal(t, ErrMissingImageName, Build(
		&BuildOptions{File: specPath},
		newCLI(t),
	))
}

func TestBuildErrMissingBuildArg(t *testing.T) {
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join("tmp", name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	defer os.RemoveAll(rootPath)
	dockerfile := `FROM alpine:3.11.6
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
	dockerfile := `FROM alpine:3.11.6
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
	dockerfile := `FROM alpine:3.11.6
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
	depDockerfile := `FROM alpine:3.11.6
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

func TestBuildDependencyModule(t *testing.T) {
	depName := "test-" + uuid.New().String()[:8]
	name := "test-" + uuid.New().String()[:8]
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
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(`dependencies: ["dep"]`),
		0644,
	))
	depDockerfile := `FROM alpine:3.11.6
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

func TestBuildDependencyDockerfile(t *testing.T) {
	depName := "test-" + uuid.New().String()[:8]
	name := "test-" + uuid.New().String()[:8]
	rootPath := filepath.Join("tmp", name)
	require.NoError(t, os.MkdirAll(rootPath, 0766))
	defer os.Remove(rootPath)
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(rootPath, "foo.txt"),
		[]byte("Hello, world!"),
		0644,
	))
	fooPath := filepath.Join(rootPath, "foo")
	require.NoError(t, os.MkdirAll(fooPath, 0766))
	// Use the dependency as a base image
	dockerfile := fmt.Sprintf(`FROM test/%s:latest
CMD ["sh", "-c", "echo \"Hello again, world\""]`, depName)
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(fooPath, "Dockerfile"),
		[]byte(dockerfile),
		0644,
	))
	specPath := filepath.Join(rootPath, "kindest.yaml")
	spec := fmt.Sprintf(`dependencies:
  - dep
build:
  name: test/%s
  dockerfile: foo/Dockerfile
`, name)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	depDockerfile := `FROM alpine:3.11.6
COPY foo.txt .
CMD ["sh", "-c", "echo \"Hello, world\""]`
	depPath := filepath.Join(rootPath, "dep")
	subdir := filepath.Join(depPath, "subdir")
	require.NoError(t, os.MkdirAll(subdir, 0766))
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(subdir, "Dockerfile"),
		[]byte(depDockerfile),
		0644,
	))
	depSpec := fmt.Sprintf(`build:
  name: test/%s
  dockerfile: subdir/Dockerfile
  context: ..
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

func TestBuildDockerignore(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		name := "test-" + uuid.New().String()[:8]
		rootPath := filepath.Join("tmp", name)
		require.NoError(t, os.MkdirAll(rootPath, 0766))
		defer os.Remove(rootPath)
		require.NoError(t, ioutil.WriteFile(
			filepath.Join(rootPath, "foo.txt"),
			[]byte("Hello, world!"),
			0644,
		))
		// Ensure the Dockerfile isn't excluded
		dockerignore := `bar.txt
Dockerfile`
		require.NoError(t, ioutil.WriteFile(
			filepath.Join(rootPath, ".dockerignore"),
			[]byte(dockerignore),
			0644,
		))
		require.NoError(t, ioutil.WriteFile(
			filepath.Join(rootPath, "foo.txt"),
			[]byte("Hello, world!"),
			0644,
		))
		require.NoError(t, ioutil.WriteFile(
			filepath.Join(rootPath, "bar.txt"),
			[]byte("Hello, world!"),
			0644,
		))
		dockerfile := fmt.Sprintf(`FROM alpine:3.11.6
COPY foo.txt .
COPY bar.txt .
CMD ["sh", "-c", "echo \"Hello again, world\""]`)
		require.NoError(t, ioutil.WriteFile(
			filepath.Join(rootPath, "Dockerfile"),
			[]byte(dockerfile),
			0644,
		))
		specPath := filepath.Join(rootPath, "kindest.yaml")
		spec := fmt.Sprintf(`
build:
  name: test/%s
`, name)
		require.NoError(t, ioutil.WriteFile(
			specPath,
			[]byte(spec),
			0644,
		))
		err := Build(
			&BuildOptions{
				File: specPath,
			},
			newCLI(t),
		)
		require.Error(t, err)
		require.Contains(t, err.Error(), "bar.txt: no such file or directory")
	})
	t.Run("dir", func(t *testing.T) {
		name := "test-" + uuid.New().String()[:8]
		rootPath := filepath.Join("tmp", name)
		require.NoError(t, os.MkdirAll(filepath.Join(rootPath, ".git"), 0766))
		require.NoError(t, os.MkdirAll(filepath.Join(rootPath, "subdir", "nested"), 0766))
		defer os.Remove(rootPath)
		require.NoError(t, ioutil.WriteFile(
			filepath.Join(rootPath, "foo.txt"),
			[]byte("Hello, world!"),
			0644,
		))
		dockerignore := `.git/`
		require.NoError(t, ioutil.WriteFile(
			filepath.Join(rootPath, ".dockerignore"),
			[]byte(dockerignore),
			0644,
		))
		require.NoError(t, ioutil.WriteFile(
			filepath.Join(rootPath, "foo.txt"),
			[]byte("Hello, world!"),
			0644,
		))
		require.NoError(t, ioutil.WriteFile(
			filepath.Join(rootPath, ".git", "bar.txt"),
			[]byte("Hello, world!"),
			0644,
		))
		require.NoError(t, ioutil.WriteFile(
			filepath.Join(rootPath, "subdir", "nested", "baz.txt"),
			[]byte("Hello, world!"),
			0644,
		))
		script := `#!/bin/bash
echo "Ensuring .git folder was successfully excluded from build context"
find .
if [ -n "$(find . | grep .git)" ]; then
	echo ".git folder was found in build context"
	exit 66
fi
bartxt=$(find . | grep bar.txt)
if [ -n "$bartxt" ]; then
	echo "bar.txt was found at ${bartxt}"
	exit 66
fi
if [ -n "$(ls | grep baz.txt)" ]; then
	echo "baz.txt was found in root dir when it should be at ./subdir/nested/baz.txt!"
	exit 66
fi
cd subdir/nested
if [ -z "$(ls | grep baz.txt)" ]; then
	echo "./subdir/nested/baz.txt was not found!"
	exit 66
fi
echo ".git folder was successfully ignored"`
		require.NoError(t, ioutil.WriteFile(
			filepath.Join(rootPath, "script"),
			[]byte(script),
			0644,
		))
		dockerfile := fmt.Sprintf(`FROM alpine:3.11.6
RUN apk add --no-cache bash
WORKDIR /app
COPY . .
RUN chmod +x ./script && ./script`)
		require.NoError(t, ioutil.WriteFile(
			filepath.Join(rootPath, "Dockerfile"),
			[]byte(dockerfile),
			0644,
		))
		specPath := filepath.Join(rootPath, "kindest.yaml")
		spec := fmt.Sprintf(`
build:
  name: test/%s
`, name)
		require.NoError(t, ioutil.WriteFile(
			specPath,
			[]byte(spec),
			0644,
		))
		err := Build(
			&BuildOptions{
				File: specPath,
			},
			newCLI(t),
		)
		require.NoError(t, err)
	})
}

func TestBuildDependencyContext(t *testing.T) {
	depName := "test-" + uuid.New().String()[:8]
	name := "test-" + uuid.New().String()[:8]
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
`, name)
	require.NoError(t, ioutil.WriteFile(
		specPath,
		[]byte(spec),
		0644,
	))
	depPath := filepath.Join(rootPath, "dep")
	subdir := filepath.Join(depPath, "subdir")
	require.NoError(t, os.MkdirAll(subdir, 0766))
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(depPath, "foo.txt"),
		[]byte("Hello, world!"),
		0644,
	))
	depDockerfile := `FROM alpine:3.11.6
COPY dep/foo.txt .
CMD ["sh", "-c", "echo \"Hello, world\""]`
	require.NoError(t, ioutil.WriteFile(
		filepath.Join(subdir, "Dockerfile"),
		[]byte(depDockerfile),
		0644,
	))
	depSpec := fmt.Sprintf(`build:
  name: test/%s
  dockerfile: subdir/Dockerfile
  context: .. # rootPath
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
	dockerfile := `FROM alpine:3.11.6
RUN echo 'Hello, world!' >> /message
CMD ["cat", "/message"]`
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
	cli := newCLI(t)
	var pool *tunny.Pool
	var isUsingCache int32
	pool = tunny.NewFunc(runtime.NumCPU(), func(payload interface{}) interface{} {
		options := payload.(*BuildOptions)
		return BuildEx(options, cli, pool, func(r io.ReadCloser) error {
			rd := bufio.NewReader(r)
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
				fmt.Println(msg.Stream)
				if strings.Contains(msg.Stream, "Using cache") {
					atomic.StoreInt32(&isUsingCache, 1)
				}
			}
			return nil
		})
	})
	defer pool.Close()
	err, _ := pool.Process(&BuildOptions{File: specPath}).(error)
	require.NoError(t, err)
	err, _ = pool.Process(&BuildOptions{File: specPath}).(error)
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&isUsingCache))
}
