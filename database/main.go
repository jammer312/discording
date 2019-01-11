package database

import (
	"database/sql"
	_ "github.com/lib/pq"
)

func Open(db_url string) (worker func(name string) *db_query_template, closer func()) {
	db, err := sql.Open("postgres", db_url)
	noerror(err)
	schema := schema_init()
	schema.deploy(db)
	templates := templates_init(db)
	worker = func(name string) *db_query_template {
		template, ok := templates[name]
		if !ok {
			panic("no template named '" + name + "'")
		}
		return &template
	}
	return worker, func() {
		templates_deinit(templates)
		noerror(db.Close()) //that way closure keeps reference to db so it stays alive until closer finishes
	}
}
