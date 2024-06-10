package server

import (
	"context"

	"github.com/andrebq/davd/internal/config"
)

type (
	Config struct {
		Address string
		Port    uint
		RootDir string
	}
)

var (
	configPrefix  = config.Key("server", "config")
	baseConfigKey = configPrefix.Sub("binds", "default_root")
)

func UpdateBaseConfig(ctx context.Context, db *config.DB, addr string, port uint, rootDir string) error {
	val := Config{
		Address: addr,
		Port:    port,
		RootDir: rootDir,
	}
	// TODO: perform some basic checks before saving them
	return db.Put(ctx, baseConfigKey, val)
}

func LoadConfig(ctx context.Context, db *config.DB) (Config, error) {
	var val Config
	err := db.Get(ctx, &val, baseConfigKey)
	return val, err
}
