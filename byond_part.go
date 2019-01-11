package main

import (
	// "bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/text/encoding/charmap"
	"log"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const SS_AUTOUPDATES_INTERVAL = 60

type server_status struct {
	server_name       string
	server_address    string
	status_table      map[string]string
	associated_embeds map[string]string //channelid -> messageid, no more than one per channel (because no reason for more than one)
	embed             discordgo.MessageEmbed
	timerchan         chan int
	/*	server_name    string
		version        string
		mode           string
		enter          string
		host           string
		players        string
		admins         string
		gamestate      string
		map_name       string
		security_level string
		round_duration string
		shuttle_mode   string
		shuttle_timer  string*/
}

const (
	GAME_STATE_STARTUP    = "0"
	GAME_STATE_PREGAME    = "1"
	GAME_STATE_SETTING_UP = "2"
	GAME_STATE_PLAYING    = "3"
	GAME_STATE_FINISHED   = "4"

	SHUTTLE_IDLE     = "0"
	SHUTTLE_RECALL   = "1"
	SHUTTLE_CALL     = "2"
	SHUTTLE_DOCKED   = "3"
	SHUTTLE_STRANDED = "4"
	SHUTTLE_ESCAPE   = "5"
	SHUTTLE_ENDGAME  = "6"
)

type embed_ft struct {
	name        string
	value_entry string
	inline      bool
}

var embed_teplate = [...]embed_ft{
	embed_ft{"Server", "server_name", false},
	embed_ft{"Version", "version", true},
	embed_ft{"Map", "map_name", true},
	embed_ft{"Address", "server_address", false},
	embed_ft{"Players", "players", true},
	embed_ft{"Admins", "admins", true},
	embed_ft{"Security level", "security_level", false},
	embed_ft{"Shuttle mode", "shuttle_mode", true},
	embed_ft{"Shuttle timer", "shuttle_timer", true},
	embed_ft{"Gamemode", "mode", false},
	embed_ft{"Game state", "gamestate", true},
	embed_ft{"Round duration", "round_duration", true},
}

//syncs with hub
var global_update_mutex sync.Mutex

func (ss *server_status) global_update() {
	global_update_mutex.Lock()
	defer global_update_mutex.Unlock()
	if ss.status_table == nil {
		ss.status_table = make(map[string]string)
	}
	resp := Byond_query_fast(ss.server_name, "status", true)
	stat := resp.String()
	if stat == "NULL" {
		//probably timeout
		return
	}
	stat = Bquery_deconvert(stat)
	stat_split := strings.Split(stat, "&")
	for i := 0; i < len(stat_split); i++ {
		tmp := strings.Split(stat_split[i], "=")
		if len(tmp) > 1 {
			ss.status_table[tmp[0]] = tmp[1]
		}
	}
	ss.update_embeds()
}

func (ss *server_status) update_embeds() {
	ss.update_embed()
	for ch, msg := range ss.associated_embeds {
		if !Discord_replace_embed(ch, msg, &(ss.embed)) {
			unbind_server_embed(ss.server_name, ch)
			log_line_runtime("unbound embed from server" + ss.server_name + " channel " + ch)
		}
	}
}

func (ss *server_status) entry(key string) string {
	if key == "server_name" {
		return ss.server_name
	}
	if key == "server_address" {
		return "byond://" + ss.server_address
	}
	val, ok := ss.status_table[key]
	if !ok {
		return "unknown"
	}
	if key == "gamestate" {
		switch val {
		case GAME_STATE_FINISHED:
			val = "FINISHED"
		case GAME_STATE_PLAYING:
			val = "PLAYING"
		case GAME_STATE_PREGAME:
			val = "PREGAME"
		case GAME_STATE_STARTUP:
			val = "STARTUP"
		case GAME_STATE_SETTING_UP:
			val = "SETTING UP"
		default:
			val = "ERR"
		}
	}
	if key == "shuttle_mode" && len(val) == 1 {
		switch val {
		case SHUTTLE_CALL:
			val = "CALLED"
		case SHUTTLE_DOCKED:
			val = "DOCKED"
		case SHUTTLE_ENDGAME:
			val = "DOCKED AT CENTCOMM"
		case SHUTTLE_ESCAPE:
			val = "ESCAPING"
		case SHUTTLE_IDLE:
			val = "IDLE"
		case SHUTTLE_RECALL:
			val = "RECALLED"
		case SHUTTLE_STRANDED:
			val = "STRANDED"
		default:
			val = "ERR"
		}
	}
	if key == "round_duration" {
		num, err := strconv.Atoi(val)
		if err == nil {
			val = fmt.Sprintf("%v hours %v mins %v secs", num/3600, (num%3600)/60, num%60)
		}
	}
	return val
}

func (ss *server_status) update_embed() {
	if ss.embed.Fields == nil {
		ss.rebuild_embed()
		return
	}
	for i := 0; i < len(embed_teplate); i++ {
		ss.embed.Fields[i].Value = ss.entry(embed_teplate[i].value_entry)
	}
}

func (ss *server_status) rebuild_embed() {
	ss.embed.Color = known_servers[ss.server_name].color
	ss.embed.Fields = make([]*discordgo.MessageEmbedField, len(embed_teplate))
	for i := 0; i < len(embed_teplate); i++ {
		ss.embed.Fields[i] = &discordgo.MessageEmbedField{
			Name:   embed_teplate[i].name,
			Value:  ss.entry(embed_teplate[i].value_entry),
			Inline: embed_teplate[i].inline,
		}
	}
}

func (ss *server_status) start_ticker() {
	if ss.timerchan != nil {
		ss.stop_ticker()
	}
	go func() {
		ticker := time.NewTicker(SS_AUTOUPDATES_INTERVAL * time.Second)
		ss.timerchan = make(chan int, 0)
		for {
			select {
			case <-ticker.C:
				ss.global_update()
			case <-ss.timerchan:
				ticker.Stop()
				ss.timerchan = nil
			}
		}
	}()
}

func (ss *server_status) stop_ticker() {
	if ss.timerchan == nil {
		return
	}
	ss.timerchan <- 1
}

var server_statuses map[string]*server_status

func populate_server_embeds() {
	for k := range server_statuses {
		delete(server_statuses, k)
	}
	defer logging_recover("pse")
	server_statuses = make(map[string]*server_status)
	var srv, chn, msg, addr string
	closure_callback := func() {
		s, ok := known_servers[srv]
		if ok {
			addr = s.addr
		}
		ss, ok := server_statuses[srv]
		if !ok {
			ss = &server_status{server_name: srv, server_address: addr}
			server_statuses[srv] = ss
		}
		if ss.associated_embeds == nil {
			ss.associated_embeds = make(map[string]string)
		}
		ss.associated_embeds[chn] = msg
	}
	db_template("select_dynembeds").query().parse(closure_callback, &srv, &chn, &msg)
	for _, ss := range server_statuses {
		ss.global_update()
	}
}

func launch_ss_tickers() {
	for _, s := range server_statuses {
		s.start_ticker()
	}
}
func stop_ss_tickers() {
	for _, s := range server_statuses {
		s.stop_ticker()
	}
}

func bind_server_embed(srv, chn, msg string) bool {
	defer logging_recover("bse")

	if db_template("update_dynembed").exec(srv, chn, msg).count() < 1 {
		db_template("create_dynembed").exec(srv, chn, msg)
	}

	ss, ok := server_statuses[srv]
	if !ok {
		var addr string
		s, ok := known_servers[srv]
		if ok {
			addr = s.addr
		}
		ss = &server_status{server_name: srv, server_address: addr}
		server_statuses[srv] = ss
	}

	if ss.associated_embeds == nil {
		ss.associated_embeds = make(map[string]string)
	}
	ss.associated_embeds[chn] = msg
	ss.global_update()
	ss.start_ticker()
	return true
}

func unbind_server_embed(srv, chn string) bool {
	defer logging_recover("use")
	ss, ok := server_statuses[srv]
	if !ok {
		return false
	}
	delete(ss.associated_embeds, chn)
	return db_template("remove_dynembed").exec(srv, chn).count() > 0
}
