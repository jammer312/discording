package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
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
