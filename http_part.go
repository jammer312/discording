package main

import (
	"encoding/json"
	"fmt"
	"html"
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
	br = Byond_query("who", false)
	fmt.Fprintln(w, br.String())
}

func safe_param(m *url.Values, param string) string {
	if len((*m)[param]) < 1 {
		return ""
	}
	return (*m)[param][0]
}

type OOCmessage struct {
	Ckey    string
	Message string
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
	case "oocmessage":
		json_data := []byte(Bquery_deconvert(safe_param(form, "data")))
		var parsed OOCmessage
		err := json.Unmarshal(json_data, &parsed)
		if err != nil {
			log.Println("json error: ", err)
			log.Println("Origin string: '", safe_param(form, "data"), "'")
			log.Println("Deconvertd string: '", Bquery_deconvert(safe_param(form, "data")), "'")
		}
		OOC_message_send("**" + parsed.Ckey + "**: " + Dsanitize(html.UnescapeString(parsed.Message)))
	default:
		log.Print(form)
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
