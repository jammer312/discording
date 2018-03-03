package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

var (
	http_webhook_key string
	port             string
)

func index_handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "" && r.Method != "GET" {
		return
	} //no POST and other shit
	//no input params, simply prints out various info
	br := Byond_query("who")
	out := br.String()
	fmt.Fprintln(w, out)
}

func webhook_handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "" && r.Method != "GET" {
		return
	} //no POST and other shit
	r.ParseForm()
	if r.Form["key"][0] != http_webhook_key {
		fmt.Fprint(w, "No command handling without password")
	}
	fmt.Fprint(w, r.Form)
}

func init() {
	http_webhook_key = os.Getenv("http_webhook_key")
	if http_webhook_key == "" {
		log.Fatalln("Failed to retrieve $http_webhook_key")
	}
	port = os.Getenv("PORT")
	if port == "" {
		log.Fatalln("Failed to retrieve $PORT")
	}
}

func main() {
	http.HandleFunc("/", index_handler)
	http.HandleFunc("/command", webhook_handler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
