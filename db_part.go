package main

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
	"os"
)

var Database *sql.DB

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

const (
	text_db_type = "text"
	int_bd_type  = "numeric"
)

type db_query_template struct {
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

var db_templates map[string]db_query_template // name -> template

func (dbqt *db_query_template) exec(values ...interface{}) *db_query_result {
	res, err := dbqt.stmt.Exec(values...)
	noerror(err)
	return &db_query_result{res}
}

func (dbqr *db_query_result) count() int64 {
	affected, err := dbqr.res.RowsAffected()
	noerror(err)
	return affected
}

func (dbqt *db_query_template) row(values ...interface{}) *db_query_row {
	return &db_query_row{dbqt.stmt.QueryRow(values...)}
}

func (dbqr *db_query_row) parse(refs ...interface{}) {
	err := dbqr.row.Scan(refs...)
	noerror(err)
}

func (dbqt *db_query_template) query(values ...interface{}) *db_query_rows {
	rows, err := dbqt.stmt.Query(values...)
	noerror(err)
	return &db_query_rows{rows}
}

func (dbqr *db_query_rows) parse(closure_callback func(), refs ...interface{}) {
	for dbqr.rows.Next() {
		terr := dbqr.rows.Scan(refs...)
		noerror(terr)
		closure_callback()
	}
}

func db_init() {
	logging_crash("db_i")
	var err error
	Database, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	noerror(err)
	schema_init()
	templates_init()
}

func db_deinit() {
	templates_deinit()
}

func templates_init() {
	defer logging_crash("tmpl_i")
	db_templates = make(map[string]db_query_template)
	prepare_template("select_known_channels", "select CHANTYPE, CHANID, SRVNAME from DISCORD_CHANNELS;")
	prepare_template("add_known_channel", "insert into DISCORD_CHANNELS values ($1, $2, $3, $4);")
	prepare_template("remove_known_channels", "delete from DISCORD_CHANNELS where CHANTYPE = $1 and SRVNAME = $2;")
	prepare_template("remove_known_channels_guild", "delete from DISCORD_CHANNELS where CHANTYPE = $1 and GUILDID = $2 and SRVNAME = $3;")
	prepare_template("update_known_channel", "update DISCORD_CHANNELS set CHANID = $2 where CHANTYPE = $1 and GUILDID = $3 and SRVNAME = $4;")
	prepare_template("create_token", "insert into DISCORD_TOKENS values ($1, $2, $3);")
	prepare_template("remove_token", "delete from DISCORD_TOKENS where TYPE = $1 and DATA = $2;")
	prepare_template("remove_token_by_id", "delete from DISCORD_TOKENS where TOKEN = $1;")
	prepare_template("select_token", "select TYPE, DATA from DISCORD_TOKENS where TOKEN = $1;")
	prepare_template("delete_user_did", "delete from DISCORD_REGISTERED_USERS where DISCORDID = $1;")
	prepare_template("delete_user_ckey", "delete from DISCORD_REGISTERED_USERS where CKEY = $1;")
	prepare_template("register_user", "insert into DISCORD_REGISTERED_USERS values ($1, $2);")
	prepare_template("select_users", "select DISCORDID, CKEY from DISCORD_REGISTERED_USERS;")
	prepare_template("select_user", "select CKEY from DISCORD_REGISTERED_USERS where DISCORDID = $1;")
	prepare_template("select_known_roles", "select GUILDID, ROLEID, ROLETYPE, SRVNAME from DISCORD_ROLES;")
	prepare_template("update_known_role", "update DISCORD_ROLES set ROLEID = $1 where GUILDID = $2 and ROLETYPE = $3 and SRVNAME = $4;")
	prepare_template("create_known_role", "insert into DISCORD_ROLES values($1, $2, $3, $4);")
	prepare_template("remove_known_role", "delete from DISCORD_ROLES where GUILDID = $1 and ROLETYPE = $2 and SRVNAME = $3;")
	prepare_template("select_bans", "select CKEY, TYPE, ADMIN, REASON from DISCORD_BANS;")
	prepare_template("select_bans_ckey", "select TYPE, ADMIN, REASON from DISCORD_BANS where CKEY = $1;")
	prepare_template("fetch_bans", "select CKEY, TYPE, PERMISSION from DISCORD_BANS;")
	prepare_template("lookup_ban", "SELECT * from DISCORD_BANS where CKEY = $1 and TYPE = $2 and ADMIN = $3;")
	prepare_template("update_ban", "update DISCORD_BANS set REASON = $1, PERMISSION = $5 where CKEY = $2 and ADMIN = $3 and TYPE = $4;")
	prepare_template("create_ban", "insert into DISCORD_BANS values($1, $2, $3, $4, $5);")
	prepare_template("remove_ban", "delete from DISCORD_BANS where CKEY = $1 and TYPE = $2 and (PERMISSION < $3::numeric or ADMIN = $4);")
	prepare_template("select_onetime_sub", "select * from DISCORD_ONETIME_SUBSCRIPTIONS where USERID=$1 and GUILDID=$2 and SRVNAME=$3;")
	prepare_template("select_onetime_subs", "select USERID, GUILDID, SRVNAME from DISCORD_ONETIME_SUBSCRIPTIONS where SRVNAME=$1;")
	prepare_template("create_onetime_sub", "insert into DISCORD_ONETIME_SUBSCRIPTIONS values($1,$2,$3);")
	prepare_template("remove_onetime_subs", "delete from DISCORD_ONETIME_SUBSCRIPTIONS where SRVNAME = $1;")
	prepare_template("select_configs", "select KEY, VALUE from app_config;")
	prepare_template("update_config", "update app_config set value=$1 where key=$2;")
	prepare_template("add_config", "insert into app_config values($1,$2);")
	prepare_template("remove_config", "delete from app_config where key=$1;")
	prepare_template("select_dynembeds", "select server, channelid, messageid from dynamic_embeds;")
	prepare_template("update_dynembed", "update dynamic_embeds set messageid=$3 where server=$1 and channelid=$2;")
	prepare_template("create_dynembed", "insert into dynamic_embeds values($1,$2,$3);")
	prepare_template("remove_dynembed", "delete from dynamic_embeds where server=$1 and channelid=$2;")
	prepare_template("select_moderators", "select ckey from discord_moderators;")
	prepare_template("add_moderator", "insert into discord_moderators values($1);")
	prepare_template("remove_moderator", "delete from discord_moderators where ckey=$1;")
}

func prepare_template(name, query string) {
	defer rise_error(name)
	stmt, err := Database.Prepare(query)
	noerror(err)
	db_templates[name] = db_query_template{stmt}
	log.Println("Adding template '" + name + "'")
}

func db_template(name string) *db_query_template {
	template, ok := db_templates[name]
	if !ok {
		panic("no template named '" + name + "'")
	}
	return &template
}

//cleanup
func templates_deinit() {
	for k, t := range db_templates {
		t.stmt.Close()
		delete(db_templates, k)
	}
}

func schema_init() {
	database_schema.tables = make([]table_schema, 0)
	add_table(table_schema{
		name: "discord_bans",
		fields: map[string]string{
			"ckey":       text_db_type,
			"admin":      text_db_type,
			"reason":     text_db_type,
			"type":       int_bd_type,
			"permission": int_bd_type,
		}})

	add_table(table_schema{
		name: "discord_channels",
		fields: map[string]string{
			"chantype": text_db_type,
			"chanid":   text_db_type,
			"guildid":  text_db_type,
			"srvname":  text_db_type,
		}})

	add_table(table_schema{
		name: "discord_onetime_subscriptions",
		fields: map[string]string{
			"userid":  text_db_type,
			"guildid": text_db_type,
			"srvname": text_db_type,
		}})

	add_table(table_schema{
		name: "discord_registered_users",
		fields: map[string]string{
			"discordid": text_db_type,
			"ckey":      text_db_type,
		}})

	add_table(table_schema{
		name: "discord_roles",
		fields: map[string]string{
			"guildid":  text_db_type,
			"roleid":   text_db_type,
			"roletype": text_db_type,
			"srvname":  text_db_type,
		}})

	add_table(table_schema{
		name: "discord_tokens",
		fields: map[string]string{
			"token": text_db_type,
			"type":  text_db_type,
			"data":  text_db_type,
		}})

	add_table(table_schema{
		name: "station_servers",
		fields: map[string]string{
			"srvname":     text_db_type,
			"srvaddr":     text_db_type,
			"commkey":     text_db_type,
			"webkey":      text_db_type,
			"admins_page": text_db_type,
			"color":       int_bd_type,
		}})

	add_table(table_schema{
		name: "app_config",
		fields: map[string]string{
			"key":   text_db_type,
			"value": text_db_type,
		}})

	add_table(table_schema{
		name: "dynamic_embeds",
		fields: map[string]string{
			"server":    text_db_type,
			"channelid": text_db_type,
			"messageid": text_db_type,
		}})

	add_table(table_schema{
		name: "discord_moderators",
		fields: map[string]string{
			"ckey": text_db_type,
		}})

	/*
		add_table(table_schema{
			name:"",
			fields: map[string]string{
				"": "",
			},})
	*/
	database_schema.deploy_db()
}

//create missing tables
//TODO: add automatic db alteration
func (dbs *db_schema) deploy_db() {
	for _, v := range dbs.tables {
		tps := v.typestring()
		cmd := "CREATE TABLE IF NOT EXISTS " + v.name + " " + tps
		_, err := Database.Exec(cmd)
		noerror(err)
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

// db table app_config {key<->value}
var config_entries map[string]string

func populate_configs() {
	defer logging_recover("p_c")
	config_entries = make(map[string]string)

	var key, val string
	closure_callback := func() {
		config_entries[key] = val
	}
	db_template("select_configs").query().parse(closure_callback, &key, &val)
}

func check_config(entry string) bool {
	_, ok := config_entries[entry]
	return ok
}

func get_config(entry string) string {
	return config_entries[entry]
}

func get_config_must(entry string) string {
	val, ok := config_entries[entry]
	if !ok {
		panic("Failed to retrieve '" + entry + "' config entry")
	}
	return val
}
func local_update_config(entry, value string) {
	config_entries[entry] = value
}
func local_remove_config(entry string) {
	delete(config_entries, entry)
}
func update_config(entry, value string) (sc bool, msg string) {
	defer logging_recover("a_c")
	msg = "some code shit happened"
	if db_template("update_config").exec(entry, value).count() < 1 {
		if db_template("add_config").exec(entry, value).count() < 1 {
			return false, "some db shit happened"
		}
		local_update_config(entry, value)
		return true, "created"
	}
	local_update_config(entry, value)
	return true, "updated"
}

func remove_config(entry string) (sc bool, msg string) {
	defer logging_recover("r_c")
	msg = "some code shit happened"
	if db_template("remove_config").exec(entry).count() < 1 {
		return false, "no such entry"
	}
	local_remove_config(entry)
	return true, "removed"
}
