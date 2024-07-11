package main

import (
	"fmt"
	"github.com/dafsic/hunter/cmd/test"
	"github.com/dafsic/hunter/version"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "hunter",
		Usage: "hunter command [args]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "version",
				Aliases: []string{"v"},
				Usage:   "Print version info",
			},
		},
		Action: Action,
		Commands: []*cli.Command{
			test.TestCmd,
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		return
	}
}

func Action(cCtx *cli.Context) error {
	buildInfo := version.Info()
	if cCtx.Bool("version") {
		fmt.Printf("VERSION:         %s\n", buildInfo.Version)
		fmt.Printf("GO_VERSION:      %s\n", buildInfo.GoVersion)
		fmt.Printf("GIT_BRANCH:      %s\n", buildInfo.GitBranch)
		fmt.Printf("COMMIT_HASH:     %s\n", buildInfo.CommitHash)
		fmt.Printf("GIT_TREE_STATE:  %s\n", buildInfo.GitTreeState)
		fmt.Printf("BUILD_TIME:      %s\n", buildInfo.BuildTime)
	} else {
		cli.ShowAppHelpAndExit(cCtx, 0)
	}
	return nil
}
