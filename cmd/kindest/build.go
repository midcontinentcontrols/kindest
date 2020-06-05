package main

import (
	"runtime"
	"time"

	"github.com/Jeffail/tunny"
	"github.com/midcontinentcontrols/kindest/pkg/kindest"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var buildArgs kindest.BuildOptions

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "",
	RunE: func(cmd *cobra.Command, args []string) error {
		start := time.Now()
		var pool *tunny.Pool
		pool = tunny.NewFunc(buildArgs.Concurrency, func(payload interface{}) interface{} {
			return kindest.BuildEx(
				payload.(*kindest.BuildOptions),
				pool,
				nil,
			)
		})
		defer pool.Close()
		err, _ := pool.Process(&buildArgs).(error)
		if err != nil {
			return err
		}
		log.Info("Build successful", zap.String("elapsed", time.Now().Sub(start).String()))
		return nil
	},
}

func init() {
	ConfigureCommand(buildCmd)
	buildCmd.PersistentFlags().StringVarP(&buildArgs.File, "file", "f", "./kindest.yaml", "Path to kindest.yaml file")
	buildCmd.PersistentFlags().StringVarP(&buildArgs.Tag, "tag", "t", "latest", "docker image tag")
	buildCmd.PersistentFlags().BoolVar(&buildArgs.NoCache, "no-cache", false, "build images from scratch")
	buildCmd.PersistentFlags().BoolVar(&buildArgs.Squash, "squash", false, "squashes newly built layers into a single new layer (docker experimental feature)")
	//buildCmd.PersistentFlags().BoolVarP(&buildArgs.Push, "push", "p", false, "push all built images")
	buildCmd.PersistentFlags().IntVarP(&buildArgs.Concurrency, "concurrency", "c", runtime.NumCPU(), "number of parallel build jobs (defaults to num cpus)")
}
