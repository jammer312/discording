package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"log"
	"os"
)

var Database *sql.DB

func init() {
	var err error
	Database, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("SQL connect error: ", err)
	}

}

type table_schema struct {
	name   string
	fields map[string]string // name -> type
}

type db_schema struct {
	tables []table_schema
}

var database_schema db_schema

func add_table(tbls table_schema) {
	database_schema.tables = append(database_schema.tables, tbls)
}

//TODO: check and correct actual field width

const (
	discordid_db_type  = "char(20)"
	byond_ckey_db_type = "char(30)"
	int_bd_type        = "numeric"
)

func schema_init() {
	database_schema.tables = make([]table_schema, 0)
	add_table(table_schema{
		name: "discord_channels",
		fields: map[string]string{
			"chantype": discordid_db_type,
			"chanid":   discordid_db_type,
			"guildid":  discordid_db_type,
			"srvname":  discordid_db_type,
		}})
	add_table(table_schema{
		name: "discord_tokens",
		fields: map[string]string{
			"token": discordid_db_type,
			"type":  discordid_db_type,
			"data":  "char(60)",
		}})
	add_table(table_schema{
		name: "discord_registered_users",
		fields: map[string]string{
			"discordid": discordid_db_type,
			"ckey":      byond_ckey_db_type,
		}})
	add_table(table_schema{
		name: "discord_roles",
		fields: map[string]string{
			"guildid":  discordid_db_type,
			"roleid":   discordid_db_type,
			"roletype": "char(20)",
			"srvname":  "char(20)",
		}})
	add_table(table_schema{
		name: "discord_bans",
		fields: map[string]string{
			"ckey":       byond_ckey_db_type,
			"reason":     "char(60)",
			"admin":      byond_ckey_db_type,
			"type":       int_bd_type,
			"permission": int_bd_type,
		}})
	add_table(table_schema{
		name: "discord_onetime_subscriptions",
		fields: map[string]string{
			"userid":  discordid_db_type,
			"guildid": discordid_db_type,
			"srvname": "char(20)",
		}})
	add_table(table_schema{
		name: "app_config",
		fields: map[string]string{
			"key":   "char(20)",
			"value": "char(20)",
		}})
	/*
		add_table(table_schema{
			name:"",
			fields: map[string]string{
				"": "",
			},})
	*/
}

// //should update/create all necessary tables and all that
// //returns success state
// func update_db() bool {

// }

type db_query_schema struct {
	compiled_select string
}

type db_query_row struct {
	row *sql.Row
}

func (dbqs *db_query_schema) compile_select(tablename string, want, know []string) {
	if len(want) < 1 {
		want = []string{"*"}
	}
	dbqs.compiled_select = "select "
	for i := 0; i < len(want); i++ {
		if i > 0 {
			dbqs.compiled_select += ", "
		}
		dbqs.compiled_select += want[i]

	}
	dbqs.compiled_select += fmt.Sprintf("from %v", tablename)
	if len(know) > 0 {
		dbqs.compiled_select += " where "
		for i := 0; i < len(know); i++ {
			if i > 0 {
				dbqs.compiled_select += " and "
			}
			dbqs.compiled_select += fmt.Sprintf("%v = $%v", know[i], i+1)
		}
	}
	dbqs.compiled_select += ";"
}

func (dbqs *db_query_schema) select_row(values ...interface{}) db_query_row {
	return db_query_row{Database.QueryRow(dbqs.compiled_select, values)}
}

func (dbqr *db_query_row) parse(refs ...interface{}) {
	err := dbqr.row.Scan(refs)
	if err != nil {
		panic(err)
	}
}

func count_query(query string) int {
	result, err := Database.Exec(query)
	if err != nil {
		log.Println("DB ERROR: failed to count: ", err)
		return -1
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return -1
	}
	return int(affected)
}
