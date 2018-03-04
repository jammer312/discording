package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
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
	br := Byond_query("status", false)
	out := br.String()
	out = strings.Replace(out, "&", "\n", -1)
	out = strings.Replace(out, "=", ": ", -1)
	out = Bquery_deconvert(out)
	fmt.Fprintln(w, out)
}

func safe_param(m *url.Values, param string) string {
	if len((*m)[param]) < 1 {
		return ""
	}
	return (*m)[param][0]
}

func webhook_handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "" && r.Method != "GET" {
		return
	} //no POST and other shit
	r.ParseForm()
	form := &r.Form
	if safe_param(form, "key") != http_webhook_key {
		fmt.Fprint(w, "No command handling without password")
		return
	}
	switch safe_param(form, "method") {
	case "message":
		OOC_message_send(safe_param(form, "content"))
	default:
		fmt.Fprint(w, form)
	}
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

func Http_server() *http.Server {
	srv := &http.Server{Addr: ":" + port}
	http.HandleFunc("/", index_handler)
	http.HandleFunc("/command", webhook_handler)
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Print("Http server error: ", err)
		}
	}()
	return srv
}
