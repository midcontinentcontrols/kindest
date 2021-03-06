package kindest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/midcontinentcontrols/kindest/pkg/logger"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

//
//
func TestModuleBuildKaniko(t *testing.T) {
	//
	//
	t.Run("BuildStatusSucceeded", func(t *testing.T) {
		name := "test-" + uuid.New().String()[:8]
		rootPath := filepath.Join("tmp", name)
		require.NoError(t, os.MkdirAll(rootPath, 0644))
		defer func() {
			require.NoError(t, os.RemoveAll(rootPath))
		}()
		specYaml := fmt.Sprintf(`build:
  name: %s`, name)
		dockerfile := `FROM alpine:3.11.6
CMD ["sh", "-c", "echo \"Hello, world\""]`
		require.NoError(t, createFiles(map[string]interface{}{
			"kindest.yaml": specYaml,
			"Dockerfile":   dockerfile,
		}, rootPath))
		log := logger.NewMockLogger(logger.NewFakeLogger())
		module, err := NewProcess(runtime.NumCPU(), log).GetModule(filepath.Join(rootPath, "kindest.yaml"))
		require.NoError(t, err)
		require.Equal(t, BuildStatusPending, module.Status())
		require.NoError(t, module.Build(&BuildOptions{Builder: "kaniko", NoPush: true}))
		require.Equal(t, BuildStatusSucceeded, module.Status())
	})
	t.Run("BuildStatusFailed", func(t *testing.T) {
		name := "test-" + uuid.New().String()[:8]
		rootPath := filepath.Join("tmp", name)
		require.NoError(t, os.MkdirAll(rootPath, 0644))
		defer func() {
			require.NoError(t, os.RemoveAll(rootPath))
		}()
		specYaml := fmt.Sprintf(`build:
  name: %s`, name)
		dockerfile := `FROM alpine:3.11.6
RUN cat foo`
		require.NoError(t, createFiles(map[string]interface{}{
			"kindest.yaml": specYaml,
			"Dockerfile":   dockerfile,
		}, rootPath))
		p := NewProcess(runtime.NumCPU(), logger.NewFakeLogger())
		module, err := p.GetModule(filepath.Join(rootPath, "kindest.yaml"))
		require.NoError(t, err)
		err = module.Build(&BuildOptions{Builder: "kaniko", NoPush: true})
		require.Error(t, err)
		require.Contains(t, err.Error(), "cat: can't open 'foo': No such file or directory")
		require.Equal(t, BuildStatusFailed, module.Status())
	})
}
