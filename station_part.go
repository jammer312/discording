package main

import (
	"time"
)

//either adds new one with defined time or adds duration to existing
func update_sdonator(server, ckey string, dur time.Duration) bool {
	defer logging_recover("update_sdonator")
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

func check_donators(server string, round int) string {
	cleanup_sdonators()
	return "" //:derp:
}
