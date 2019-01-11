package database

import (
	"database/sql"
	"github.com/jammer312/discording/errors"
)

func templates_init(db *sql.DB) map[string]db_query_template {
	defer errors.LogCrash("database templates_init()")
	db_templates := make(map[string]db_query_template)
	prepare_template := func(name, query string) {
		defer errors.Rise(name)
		stmt, err := db.Prepare(query)
		errors.Deny(err)
		db_templates[name] = db_query_template{stmt}
	}
	prepare_template("select_known_channels", "select CHANTYPE, CHANID, SRVNAME from DISCORD_CHANNELS;")
	prepare_template("add_known_channel", "insert into DISCORD_CHANNELS values ($1, $2, $3, $4);")
	prepare_template("remove_known_channels", "delete from DISCORD_CHANNELS where CHANTYPE = $1 and SRVNAME = $2;")
	prepare_template("remove_known_channels_guild", "delete from DISCORD_CHANNELS where CHANTYPE = $1 and GUILDID = $2 and SRVNAME = $3;")
	prepare_template("remove_known_channels_id", "delete from DISCORD_CHANNELS where CHANID = $1;")
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

	//station donatery
	prepare_template("cleanup_station_donators", "delete from station_donators where uptotime<$1;")
	prepare_template("check_station_donators", "select ckey from station_donators where server=$1 and next_round<=$2;")
	prepare_template("check_station_donator_next_round", "select next_round from station_donators where server=$1 and ckey=$2;")
	prepare_template("expend_station_donator", "update station_donators set next_round=$3 where server=$1 and ckey=$2;")
	prepare_template("update_station_donators", "update station_donators set uptotime=(uptotime+$3) where server=$1 and ckey=$2;")
	prepare_template("insert_station_donators", "insert into station_donators values($1,$2,$3,-1);")
	prepare_template("list_station_donators", "select ckey,uptotime,next_round from station_donators where server=$1;")
	return db_templates
}

//cleanup
func templates_deinit(db_templates map[string]db_query_template) {
	for k, t := range db_templates {
		t.stmt.Close()
		delete(db_templates, k)
	}
}

func schema_init() *db_schema {
	var database_schema db_schema
	database_schema.tables = make([]table_schema, 0)
	add_table := func(tbls table_schema) {
		database_schema.tables = append(database_schema.tables, tbls)
	}
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
	add_table(table_schema{
		name: "station_donators",
		fields: map[string]string{
			"server":     text_db_type,
			"ckey":       text_db_type,
			"uptotime":   int_bd_type,
			"next_round": int_bd_type,
		}})
	/*
		add_table(table_schema{
			name:"",
			fields: map[string]string{
				"": "",
			},})
	*/
	return &database_schema
}
