package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sync"

	"github.com/vmihailenco/msgpack/v5"
	_ "modernc.org/sqlite"
)

type (
	DB struct {
		conn *sql.DB

		getkey *sql.Stmt
		putkey *sql.Stmt
		delKey *sql.Stmt
		getAll *sql.Stmt

		closer struct {
			err error
			fn  func()
		}
	}

	K struct {
		k string
	}
)

func Key(txt ...string) K {
	return K{path.Join(txt...)}
}

func (k K) Sub(txt ...string) K {
	return K{k: path.Join(k.k, path.Join(txt...))}
}

func NotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func Open(ctx context.Context, dir string) (*DB, error) {
	db := filepath.Join(dir, "config.db")
	conn, err := sql.Open("sqlite", fmt.Sprintf("file:%v", db))
	if err != nil {
		return nil, err
	}
	err = conn.PingContext(ctx)
	if err != nil {
		conn.Close()
		return nil, err
	}
	store := &DB{conn: conn}
	store.closer.fn = sync.OnceFunc(func() {
		store.closer.err = store.doClose()
	})
	return store, store.init(ctx)
}

func (d *DB) init(ctx context.Context) error {
	_, err := d.conn.ExecContext(ctx, `
	create table if not exists davd_config_store(
		key text not null,
		value blob not null,
		primary key(key)
	);
	`)
	if err != nil {
		return err
	}
	d.getkey, err = d.conn.PrepareContext(ctx, `select value from davd_config_store where key = ?`)
	if err != nil {
		d.Close()
		return err
	}
	d.getAll, err = d.conn.PrepareContext(ctx, `select key, value from davd_config_store where key like ?`)
	if err != nil {
		d.Close()
		return err
	}
	d.putkey, err = d.conn.PrepareContext(ctx, `insert into davd_config_store (key, value) values (?, ?) on conflict (key) do update set value = excluded.value`)
	if err != nil {
		d.Close()
		return err
	}
	d.delKey, err = d.conn.PrepareContext(ctx, `delete from davd_config_store where key like ?`)
	if err != nil {
		d.Close()
		return err
	}

	return nil
}

func (d *DB) doClose() error {
	if d.getkey != nil {
		d.getkey.Close()
	}
	if d.getAll != nil {
		d.getAll.Close()
	}
	if d.putkey != nil {
		d.putkey.Close()
	}
	if d.delKey != nil {
		d.delKey.Close()
	}
	return d.conn.Close()
}

func (d *DB) Close() error {
	d.closer.fn()
	return d.closer.err
}

func (d *DB) Get(ctx context.Context, out any, key K) error {
	var buf []byte
	err := d.getkey.QueryRowContext(ctx, key.k).Scan(&buf)
	if err != nil {
		return err
	}
	return msgpack.Unmarshal(buf, out)
}

func (d *DB) DelPrefix(ctx context.Context, key K) (int64, error) {
	rows, err := d.delKey.ExecContext(ctx, fmt.Sprintf("%v%%", key.k))
	if err != nil {
		return 0, err
	}
	return rows.RowsAffected()
}

func (d *DB) Put(ctx context.Context, key K, val any) error {
	buf, err := msgpack.Marshal(val)
	if err != nil {
		return err
	}
	_, err = d.putkey.ExecContext(ctx, key.k, buf)
	return err
}
