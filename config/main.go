package config

import (
	"fmt"
	"github.com/jammer312/discording/database"
	"github.com/jammer312/discording/errors"
)

func Create(db_worker func(name string) *database.Db_query_template) (
	Get, MustGet func(string) string,
	Check func(string) bool,
	Update func(string, string) (bool, string),
	Remove func(string) (bool, string)) {

	config_entries := make(map[string]string)

	Get = func(entry string) string {
		return config_entries[entry]
	}

	MustGet = func(entry string) string {
		val, ok := config_entries[entry]
		errors.Assert(ok, fmt.Sprintf("Failed to retrieve '%v' config entry", entry))
		return val
	}

	Check = func(entry string) bool {
		_, ok := config_entries[entry]
		return ok
	}

	local_update := func(entry, value string) {
		config_entries[entry] = value
	}
	local_remove := func(entry string) {
		delete(config_entries, entry)
	}
	local_soft_update := func(entry, value string) {
		if _, ok := config_entries[entry]; !ok {
			config_entries[entry] = value
		}
	}

	Update = func(entry, value string) (sc bool, msg string) {
		defer errors.LogRecover("config Update")
		msg = "local problem"
		if db_worker("update_config").Exec(entry, value).Count() < 1 {
			if db_worker("add_config").Exec(entry, value).Count() < 1 {
				return false, "database problem"
			}
			local_update(entry, value)
			return true, "created"
		}
		local_update(entry, value)
		return true, "updated"
	}

	Remove = func(entry string) (sc bool, msg string) {
		defer errors.LogRecover("config Remove")
		msg = "local problem"
		if db_worker("remove_config").Exec(entry).Count() < 1 {
			return false, "no such entry"
		}
		local_remove(entry)
		return true, "removed"
	}

	//populate configs
	var key, val string
	closure_callback := func() {
		config_entries[key] = val
	}
	db_worker("select_configs").Query().Parse(closure_callback, &key, &val)

	//local config (overrided by db) //it shouldn't be there but whatever
	local_soft_update("st_d_traitor_cooldown", "4")
	local_soft_update("st_d_changeling_cooldown", "4")
	local_soft_update("st_d_wizard_cooldown", "5")
	local_soft_update("st_d_devil_cooldown", "3")
	local_soft_update("st_d_revenant_cooldown", "3")
	//

	return Get, MustGet, Check, Update, Remove
}
