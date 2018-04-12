package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func logging_crash(ctx string) {
	if r := recover(); r != nil {
		log.Fatalln("ERRF: ["+ctx+"]:", r)
	}
}

func logging_recover(ctx string) {
	if r := recover(); r != nil {
		log.Println("ERR: ["+ctx+"]:", r)
	}
}

func recovering_callback(callback func()) {
	if r := recover(); r != nil {
		callback()
	}
}

func logging_pass(ctx string) {
	if r := recover(); r != nil {
		log.Println("ERR: ["+ctx+"]:", r)
		panic(r)
	}
}

func rise_error(app string) {
	if r := recover(); r != nil {
		panic(fmt.Sprintf("[%v]:%v", app, r))
	}
}

func noerror(err error) {
	if err != nil {
		panic(err)
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
	/*	loc, err := time.LoadLocation("Europe/Moscow")
		if err != nil {
			panic(err)
		}*/
	get_time = func() string {
		return time.Now().String()
	}
}

func main() {
	db_init()
	init_time()
	discord_init()
	populate_servers()
	Dopen()              //start discord
	srv := Http_server() //start web server
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc        //wait for SIGINT or kinda it
	Dclose()    //stop discord
	db_deinit() //clean db templates
	//graceful shutdown for web server
	if err := srv.Shutdown(nil); err != nil {
		log.Fatal("Failed to shutdown webserver: ", err)
	}
	log.Println("Stoped correctly")
}
