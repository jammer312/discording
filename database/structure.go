package database

import (
	"database/sql"
	"github.com/jammer312/discording/errors"
)

const (
	text_db_type = "text"
	int_bd_type  = "numeric"
)

type table_schema struct {
	name   string
	fields map[string]string // name -> type
}

type db_schema struct {
	tables []table_schema
}

type Db_query_template struct {
	stmt *sql.Stmt
}

type db_query_result struct {
	res sql.Result
}

type db_query_row struct {
	row *sql.Row
}

type db_query_rows struct {
	rows *sql.Rows
}

func (dbqt *Db_query_template) Exec(values ...interface{}) *db_query_result {
	res, err := dbqt.stmt.Exec(values...)
	errors.Deny(err)
	return &db_query_result{res}
}

func (dbqr *db_query_result) Count() int64 {
	affected, err := dbqr.res.RowsAffected()
	errors.Deny(err)
	return affected
}

func (dbqt *Db_query_template) Row(values ...interface{}) *db_query_row {
	return &db_query_row{dbqt.stmt.QueryRow(values...)}
}

func (dbqr *db_query_row) Parse(refs ...interface{}) {
	err := dbqr.row.Scan(refs...)
	errors.Deny(err)
}

func (dbqt *Db_query_template) Query(values ...interface{}) *db_query_rows {
	rows, err := dbqt.stmt.Query(values...)
	errors.Deny(err)
	return &db_query_rows{rows}
}

func (dbqr *db_query_rows) Parse(closure_callback func(), refs ...interface{}) {
	for dbqr.rows.Next() {
		terr := dbqr.rows.Scan(refs...)
		errors.Deny(terr)
		closure_callback()
	}
}

//create missing tables
//TODO: add automatic db alteration
func (dbs *db_schema) deploy(db *sql.DB) {
	for _, v := range dbs.tables {
		tps := v.typestring()
		cmd := "CREATE TABLE IF NOT EXISTS " + v.name + " " + tps
		_, err := db.Exec(cmd)
		errors.Deny(err)
	}
}

func (tbs *table_schema) typestring() string {
	ret := "("
	first := true
	for k, v := range tbs.fields {
		if !first {
			ret += ", "
		}
		ret += k + " " + v
		first = false
	}
	ret += ")"
	return ret
}
