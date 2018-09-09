package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

//either adds new one with defined time or adds duration to existing
func update_sdonator(server, ckey string, dur time.Duration) bool {
	defer logging_recover("update_sdonator")
	ckey = ckey_simplifier(ckey)
	state := db_template("update_station_donators").exec(server, ckey, int64(dur.Seconds())).count() > 0 || db_template("insert_station_donators").exec(server, ckey, time.Now().Add(dur).Unix()).count() > 0 //duration is in nanoseconds, we want seconds for unix format
	if !state {
		panic("add_sdonator failed horribly")
	}
	cleanup_sdonators()
	return true
}

func cleanup_sdonators() {
	defer logging_recover("cleanup_sd")
	db_template("cleanup_station_donators").exec(time.Now().Unix())
}

func expend_donator(server, keyname string, curround int, role string, friends int, hunter bool) {
	defer logging_recover("expend_donator")
	ckey := ckey_simplifier(keyname)
	cur_next_round := 0
	db_template("check_station_donator_next_round").row(server, ckey).parse(&cur_next_round)
	if curround > cur_next_round {
		cur_next_round = curround
	}
	var cooldown int
	var err error
	switch role {
	case "traitor":
		cooldown, err = strconv.Atoi(get_config_must("st_d_traitor_cooldown"))
		noerror(err)
	case "changeling":
		cooldown, err = strconv.Atoi(get_config_must("st_d_changeling_cooldown"))
		noerror(err)
	case "wizard":
		cooldown, err = strconv.Atoi(get_config_must("st_d_wizard_cooldown"))
		noerror(err)
	case "devil":
		cooldown, err = strconv.Atoi(get_config_must("st_d_devil_cooldown"))
		noerror(err)
	case "revenant":
		cooldown, err = strconv.Atoi(get_config_must("st_d_revenant_cooldown"))
		noerror(err)
	}
	if friends > 0 {
		cooldown -= 1
		if hunter {
			cooldown -= 1
		}
	}
	cur_next_round += cooldown
	db_template("expend_donator").exec(server, ckey, cur_next_round)
}

func check_donators(server string, round int) string {
	defer logging_recover("check_donators")
	cleanup_sdonators()
	res := make([]string, 0)
	tmp := ""
	callback := func() {
		res = append(res, tmp)
	}
	db_template("check_station_donators").query(server, round).parse(callback, &tmp)
	return strings.Join(res, " ")
}

func list_donators(server string) string {
	defer logging_recover("list_donators")
	ret := ""
	var ckey string
	var uptotime int64
	closure_callback := func() {
		time_secs := uptotime - time.Now().Unix()
		time_minutes := time_secs / 60
		time_hours := time_minutes / 60
		time_days := time_hours / 24
		time_secs %= 60
		time_minutes %= 60
		time_hours %= 24
		ret += fmt.Sprintf("`%v` -> **%vd %vh %vm %vs**\n", ckey, time_days, time_hours, time_minutes, time_secs)
	}
	db_template("list_station_donators").query(server).parse(closure_callback, &ckey, &uptotime)
	return ret
}
