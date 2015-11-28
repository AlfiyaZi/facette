package backend

import (
	"fmt"
	"reflect"
	"time"

	"github.com/facette/facette/pkg/config"
	_ "github.com/mattn/go-sqlite3"
)

type sqlite3Driver struct{}

func (d sqlite3Driver) getBindVar(i int) string {
	return "?"
}

func (d sqlite3Driver) makeDSN(c map[string]interface{}) (string, error) {
	return config.GetString(c, "path", true)
}

func (d sqlite3Driver) quoteName(s string) string {
	return fmt.Sprintf("%q", s)
}

func (d sqlite3Driver) sqlSchema() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS scales (
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			created TEXT NOT NULL DEFAULT now,
			modified TEXT NOT NULL DEFAULT now,
			value NUMERIC NOT NULL,
			CONSTRAINT "pk_scales" PRIMARY KEY (id),
			CONSTRAINT "un_scales_name" UNIQUE (name),
			CONSTRAINT "un_scales_value" UNIQUE (value)
		)`,
		`CREATE TABLE IF NOT EXISTS units (
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			created TEXT NOT NULL DEFAULT now,
			modified TEXT NOT NULL DEFAULT now,
			label TEXT NOT NULL,
			CONSTRAINT "pk_units" PRIMARY KEY (id),
			CONSTRAINT "un_units_name" UNIQUE (name),
			CONSTRAINT "un_units_value" UNIQUE (label)
		)`,
		`CREATE TABLE IF NOT EXISTS sourcegroups (
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			created TEXT NOT NULL DEFAULT now,
			modified TEXT NOT NULL DEFAULT now,
			entries TEXT NOT NULL,
			CONSTRAINT "pk_sourcegroups" PRIMARY KEY (id),
			CONSTRAINT "un_sourcegroups_name" UNIQUE (name)
		)`,
		`CREATE TABLE IF NOT EXISTS metricgroups (
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			created TEXT NOT NULL DEFAULT now,
			modified TEXT NOT NULL DEFAULT now,
			entries TEXT NOT NULL,
			CONSTRAINT "pk_metricgroups" PRIMARY KEY (id),
			CONSTRAINT "un_metricgroups_name" UNIQUE (name)
		)`,
		`CREATE TABLE IF NOT EXISTS graphs (
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			created TEXT NOT NULL DEFAULT now,
			modified TEXT NOT NULL DEFAULT now,
			series TEXT,
			link TEXT,
			attributes TEXT,
			options TEXT,
			template INTEGER NOT NULL DEFAULT 0,
			CONSTRAINT "pk_graphs" PRIMARY KEY (id),
			CONSTRAINT "fk_graphs_link" FOREIGN KEY (link) REFERENCES graphs (id)
				ON DELETE CASCADE ON UPDATE CASCADE,
			CONSTRAINT "un_graphs_name" UNIQUE (name),
			CONSTRAINT "ck_graphs_entry" CHECK ((series IS NOT NULL AND link IS NULL AND attributes IS NULL) OR
				(series IS NULL AND template = 0 AND link IS NOT NULL AND attributes IS NOT NULL))
		)`,
		`CREATE TABLE IF NOT EXISTS collections (
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			created TEXT NOT NULL DEFAULT now,
			modified TEXT NOT NULL DEFAULT now,
			CONSTRAINT "pk_collections" PRIMARY KEY (id),
			CONSTRAINT "un_collections_name" UNIQUE (name)
		)`,
		`CREATE TABLE IF NOT EXISTS collections_graphs (
			collection_id TEXT NOT NULL,
			graph_id TEXT NOT NULL,
			options TEXT,
			CONSTRAINT "pk_collections_graphs" PRIMARY KEY (collection_id, graph_id),
			CONSTRAINT "fk_collections_graphs_collection_id" FOREIGN KEY (collection_id) REFERENCES collections (id)
				ON DELETE CASCADE ON UPDATE CASCADE,
			CONSTRAINT "fk_collections_graphs_graph_id" FOREIGN KEY (graph_id) REFERENCES graphs (id)
				ON DELETE CASCADE ON UPDATE CASCADE
		)`,
	}
}

func (d sqlite3Driver) transformValue(rv, value reflect.Value) (reflect.Value, error) {
	switch rv.Kind() {
	case reflect.Bool:
		return reflect.ValueOf(value.Interface().(int64) != 0), nil

	case reflect.Struct:
		if _, ok := rv.Interface().(time.Time); ok {
			t, err := time.Parse("2006-01-02 15:04:05Z07:00", string(value.Interface().([]byte)))
			if err != nil {
				return reflect.ValueOf(nil), err
			}

			return reflect.ValueOf(t), nil
		}
	}

	return value, ErrNotTransformable
}