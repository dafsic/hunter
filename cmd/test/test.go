package test

import (
	"time"

	"github.com/dafsic/hunter/config"
	"github.com/dafsic/hunter/exchange/binance"
	jcdhttp "github.com/dafsic/hunter/pkg/http"
	"github.com/dafsic/hunter/pkg/log"
	"github.com/dafsic/hunter/pkg/ws"
	"github.com/dafsic/hunter/strategy/test"

	"github.com/urfave/cli/v2"

	"go.uber.org/fx"
)

var TestCmd = &cli.Command{
	Name:  "test",
	Usage: "run test strategy",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			EnvVars: []string{"HUNTER_CFG"},
			Value:   "./config.toml",
			Usage:   "Load configuration from `FILE`",
		},
	},
	Action: func(cctx *cli.Context) error {
		path := cctx.String("config")
		cfg, err := config.NewCfg(path)
		if err != nil {
			return err
		}

		fx.New(
			fx.Supply(cfg),
			ws.ModuleFx,
			log.ModuleFx,
			jcdhttp.ModuleFx,
			test.ModuleFx,
			binance.ModuleFx,
			fx.StartTimeout(time.Second*30),
			fx.StopTimeout(time.Second*30),
			//fx.NopLogger,
		).Run()

		return nil
	},
}
