package server

import (
	"context"
	"log/slog"
	"regexp"
	"sort"
)

type (
	DynamicBindings struct {
		Entries map[string]string
	}
)

func UpdateDynamicBinds(ctx context.Context, environ func() []string, expandEnv func(string) string) (*DynamicBindings, error) {
	vars := environ()
	sort.Strings(vars)

	varnameRE := regexp.MustCompile("^DAVD_DYNBIND_([A-Z]+)=(.*)$")
	bindVal := regexp.MustCompile("^([a-z0-9]+):(.*)$")
	dynbind := DynamicBindings{
		Entries: make(map[string]string),
	}
	for _, v := range vars {
		matches := varnameRE.FindAllStringSubmatch(v, -1)
		if len(matches) == 1 {
			value := matches[0][2]
			nameAndPath := bindVal.FindAllStringSubmatch(expandEnv(value), -1)
			if len(nameAndPath) == 1 {
				bindName := nameAndPath[0][1]
				bindPath := nameAndPath[0][2]
				slog.Info("Found dynamic binding", "name", bindName, "path", bindPath)
				dynbind.Entries[nameAndPath[0][1]] = nameAndPath[0][2]
			}
		}
	}
	return &dynbind, nil
}
