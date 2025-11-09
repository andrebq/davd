package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"

	"github.com/andrebq/davd/internal/config"
	"github.com/andrebq/davd/internal/server"
	"github.com/urfave/cli/v2"
)

func main() {
	var enableDebug bool = false

	var configdb *config.DB
	configDir := "."
	app := cli.App{
		Name: "davd",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config-dir",
				Usage:       "Directory where davd configuration lives",
				EnvVars:     []string{"DAVD_SERVER_CONFIG_DIR"},
				Destination: &configDir,
				Value:       configDir,
			},
			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "Enable debug logging",
				EnvVars:     []string{"DAVD_DEBUG"},
				Destination: &enableDebug,
				Value:       enableDebug,
				Hidden:      false,
			},
		},
		Before: func(ctx *cli.Context) error {
			ll := slog.LevelInfo
			if enableDebug {
				ll = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: ll,
			})))
			var err error
			configdb, err = config.Open(ctx.Context, configDir, os.Getenv)
			if err != nil {
				return err
			}
			return nil
		},
		Commands: []*cli.Command{
			serverCmd(&configdb),
			authCmd(&configdb),
		},
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}

func authCmd(db **config.DB) *cli.Command {
	return &cli.Command{
		Name: "auth",
		Subcommands: []*cli.Command{
			authUserCmd(db),
		},
	}
}

func authUserCmd(db **config.DB) *cli.Command {
	var username string
	var permissions cli.StringSlice
	var canWrite bool
	return &cli.Command{
		Name: "user",
		Subcommands: []*cli.Command{
			{
				Name:        "add",
				Description: "Add a new user, password is read from stdin all leading/trailing whitespace gets removed!",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Usage: "username", Required: true, Destination: &username},
				},
				Action: func(ctx *cli.Context) error {
					passwd, err := io.ReadAll(os.Stdin)
					if err != nil {
						return err
					}
					passwd = bytes.TrimSpace(passwd)
					return (*db).UpsertUser(username, string(passwd))
				},
			},
			{
				Name:        "update-permission",
				Description: "Update the given profile with a new set of permissions",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Usage: "username", Required: true, Destination: &username},
					&cli.StringSliceFlag{Name: "prefix", Aliases: []string{"p"}, Usage: "One or more prefixes that the user can access", Destination: &permissions},
					&cli.BoolFlag{Name: "can-write", Aliases: []string{"w"}, Usage: "Indicates if the user can write (applies to previous paths as well)", Destination: &canWrite},
				},
				Action: func(ctx *cli.Context) error {
					var perms []config.Permission
					for _, p := range permissions.Value() {
						perms = append(perms, config.Permission{
							Prefix:  p,
							Reader:  true,
							Writer:  canWrite,
							Execute: false,
						})
					}
					return (*db).UpdatePermissions(username, perms)
				},
			},
			{
				Name:        "list-permissions",
				Description: "Return the list of paths that a given user can acces",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Usage: "username", Required: true, Destination: &username},
				},
				Action: func(ctx *cli.Context) error {
					user, err := (*db).FindUser(username)
					if err != nil {
						return err
					}
					return json.NewEncoder(ctx.App.Writer).Encode(user.Permissions)
				},
			},
		},
	}
}

func serverCmd(db **config.DB) *cli.Command {
	return &cli.Command{
		Name:  "server",
		Flags: []cli.Flag{},
		Subcommands: []*cli.Command{
			serverRunCmd(db),
		},
	}
}

func serverRunCmd(db **config.DB) *cli.Command {
	var addr string
	var port uint
	var rootDir string
	var adminToken string
	var hostAndPort string
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
		Before: func(ctx *cli.Context) error {
			hostAndPort = net.JoinHostPort(addr, strconv.FormatUint(uint64(port), 10))
			return nil
		},
		Action: func(ctx *cli.Context) error {
			created, err := (*db).InitialSetup()
			if err != nil {
				return err
			}
			if created {
				slog.Info("Root user created!")
			} else {
				slog.Info("Root user already present, DAVD_ADMIN_TOKEN was ignored")
			}
			return server.Run(ctx.Context, *db, hostAndPort, server.Environ{
				Entries: os.Environ,
				Expand:  os.ExpandEnv,
			})
		},
	}
}
