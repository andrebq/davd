package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"

	"github.com/andrebq/davd/internal/auth"
	"github.com/andrebq/davd/internal/config"
	"github.com/andrebq/davd/internal/server"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.App{
		Name: "davd",
		Flags: []cli.Flag{
			&cli.StringFlag{},
		},
		Commands: []*cli.Command{
			serverCmd(),
		},
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}

func serverCmd() *cli.Command {
	var configdb *config.DB

	configDir := "."

	return &cli.Command{
		Name: "server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config-dir",
				Usage:       "Directory where davd configuration lives",
				EnvVars:     []string{"DAVD_SERVER_CONFIG_DIR"},
				Destination: &configDir,
				Value:       configDir,
			},
		},
		Before: func(ctx *cli.Context) error {
			var err error
			configdb, err = config.Open(ctx.Context, configDir)
			if err != nil {
				return err
			}
			return nil
		},
		Subcommands: []*cli.Command{
			serverRunCmd(&configdb),
		},
	}
}

func serverRunCmd(db **config.DB) *cli.Command {
	var addr string
	var port uint
	var rootDir string
	var adminToken string
	return &cli.Command{
		Name:  "run",
		Usage: "Run the HTTP server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "addr",
				Usage:       "Address to listen for incoming connections",
				EnvVars:     []string{"DAVD_ADDR"},
				Required:    true,
				Value:       addr,
				Destination: &addr,
			},
			&cli.UintFlag{
				Name:        "port",
				Usage:       "Port to listen for incoming connections",
				EnvVars:     []string{"DAVD_PORT"},
				Required:    true,
				Value:       port,
				Destination: &port,
			},
			&cli.StringFlag{
				Name:        "root-dir",
				Usage:       "Path to the default WebDAV root",
				Required:    true,
				EnvVars:     []string{"DAVD_ROOT_DIR"},
				Value:       rootDir,
				Destination: &rootDir,
			},
			&cli.StringFlag{
				Name:        "admin-token",
				Usage:       "Initial token to setup admin account, discarded if the root user already exists",
				Required:    true,
				EnvVars:     []string{"DAVD_ADMIN_TOKEN"},
				DefaultText: "<redacted>",
				Destination: &adminToken,
			},
		},
		Action: func(ctx *cli.Context) error {
			created, initRootErr := auth.InitRoot(ctx.Context, *db, adminToken)
			if initRootErr != nil {
				return initRootErr
			}
			if created {
				slog.Info("Root user created!")
			} else {
				slog.Info("Root user already present, DAVD_ADMIN_TOKEN was ignored")
			}
			err := server.UpdateBaseConfig(ctx.Context, *db, addr, port, rootDir)
			if err != nil {
				return err
			}
			return server.Run(ctx.Context, *db, server.Environ{
				Entries: os.Environ,
				Expand:  os.ExpandEnv,
			})
		},
	}
}
