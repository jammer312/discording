package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func logging_recover(ctx string) {
	if r := recover(); r != nil {
		log.Println("ERR: ["+ctx+"]:", r)
	}
}

var known_servers map[string]server //server name -> server struct

type server struct {
	name        string //simply identifier
	addr        string //for byond queries
	comm_key    string //password
	webhook_key string //password for bot
	admins_page string //where to get admins from
	color       int    //for embeds
}

func add_server(server server) {
	known_servers[server.name] = server
}

func check_server(server string) bool {
	_, ok := known_servers[server]
	return ok
}

func init() {
	known_servers = make(map[string]server)
}

func populate_servers() {
	for k := range known_servers {
		delete(known_servers, k)
	}
	defer logging_recover("DB PS ERR:")
	rows, err := Database.Query("select SRVNAME, SRVADDR, COMMKEY, WEBKEY, ADMINS_PAGE, COLOR from STATION_SERVERS ;")
	if err != nil {
		panic(err)
	}
	for rows.Next() {
		var srvname, srvaddr, commkey, webkey, admp string
		var clr int
		if terr := rows.Scan(&srvname, &srvaddr, &commkey, &webkey, &admp, &clr); terr != nil {
			panic(terr)
		}
		srvname = trim(srvname)
		srvaddr = trim(srvaddr)
		commkey = trim(commkey)
		webkey = trim(webkey)
		admp = trim(admp)
		add_server(server{
			name:        srvname,
			addr:        srvaddr,
			comm_key:    commkey,
			webhook_key: webkey,
			admins_page: admp,
			color:       clr,
		})
	}
}

var get_time func() string

func init_time() {
	defer logging_recover("init_time")
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		panic(err)
	}
	get_time = func() string {
		return time.Now()
	}
}

func main() {
	init_time()
	populate_servers()
	Dopen()              //start discord
	srv := Http_server() //start web server
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc     //wait for SIGINT or kinda it
	Dclose() //stop discord
	//graceful shutdown for web server
	if err := srv.Shutdown(nil); err != nil {
		log.Fatal("Failed to shutdown webserver: ", err)
	}
	log.Println("Stoped correctly")
}
