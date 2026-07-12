package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var fileNamePattern = regexp.MustCompile(`^([0-9]+)_([a-zA-Z0-9_-]+)\.sql$`)

type Execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

type Migration struct {
	Version  int64
	Name     string
	Path     string
	SQL      string
	Checksum string
}

func Load(fsys embed.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	result := make([]Migration, 0, len(entries))
	seen := map[int64]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := fileNamePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", entry.Name(), err)
		}
		if previous, ok := seen[version]; ok {
			return nil, fmt.Errorf("duplicate migration version %d in %s and %s", version, previous, entry.Name())
		}
		seen[version] = entry.Name()

		filePath := path.Join(dir, entry.Name())
		raw, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", filePath, err)
		}
		result = append(result, Migration{
			Version:  version,
			Name:     strings.TrimSuffix(matches[2], ".sql"),
			Path:     filePath,
			SQL:      string(raw),
			Checksum: checksum(raw),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result, nil
}

func Run(ctx context.Context, pool *pgxpool.Pool, service string, list []Migration) error {
	if _, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	service TEXT NOT NULL,
	version BIGINT NOT NULL,
	name TEXT NOT NULL,
	checksum TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (service, version)
);
`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	for _, migration := range list {
		if err := apply(ctx, pool, service, migration); err != nil {
			return err
		}
	}
	return nil
}

func apply(ctx context.Context, pool *pgxpool.Pool, service string, migration Migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", migration.Version, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var existingChecksum string
	err = tx.QueryRow(ctx, `
SELECT checksum
FROM schema_migrations
WHERE service = $1 AND version = $2
	`, service, migration.Version).Scan(&existingChecksum)
	if err == nil {
		if existingChecksum != migration.Checksum {
			return fmt.Errorf("migration %s/%d checksum mismatch", service, migration.Version)
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check migration %s/%d: %w", service, migration.Version, err)
	}

	if _, err := tx.Exec(ctx, migration.SQL); err != nil {
		return fmt.Errorf("apply migration %s/%d %s: %w", service, migration.Version, migration.Name, err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO schema_migrations (service, version, name, checksum)
VALUES ($1, $2, $3, $4)
`, service, migration.Version, migration.Name, migration.Checksum); err != nil {
		return fmt.Errorf("record migration %s/%d: %w", service, migration.Version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s/%d: %w", service, migration.Version, err)
	}
	return nil
}

func checksum(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
