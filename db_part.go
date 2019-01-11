// db table app_config {key<->value}
var config_entries map[string]string

func def_config_init() {
	local_soft_update_config("st_d_traitor_cooldown", "4")
	local_soft_update_config("st_d_changeling_cooldown", "4")
	local_soft_update_config("st_d_wizard_cooldown", "5")
	local_soft_update_config("st_d_devil_cooldown", "3")
	local_soft_update_config("st_d_revenant_cooldown", "3")

}

func populate_configs() {
	defer logging_recover("p_c")
	config_entries = make(map[string]string)
	var key, val string
	closure_callback := func() {
		config_entries[key] = val
	}
	def_config_init()
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
func local_soft_update_config(entry, value string) {
	if _, ok := config_entries[entry]; !ok {
		config_entries[entry] = value
	}
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
