package main

import (
	"encoding/json"
	"fmt"
	"github.com/grokify/html-strip-tags-go"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var (
	http_webhook_key     string
	port                 string
	admin_retrieval_page string
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

type message struct {
	Ckey    string
	Message string
}

type token struct {
	Ckey  string
	Token string
}

type roundstatus struct {
	Status string
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
		var parsed message
		err := json.Unmarshal(json_data, &parsed)
		if err != nil {
			log.Println("json error: ", err)
		}
		Discord_message_send("ooc", "OOC:", parsed.Ckey, html.UnescapeString(parsed.Message))
	case "asaymessage":
		json_data := []byte(Bquery_deconvert(safe_param(form, "data")))
		var parsed message
		err := json.Unmarshal(json_data, &parsed)
		if err != nil {
			log.Println("json error: ", err)
		}
		Discord_message_send("admin", "ASAY:", parsed.Ckey, html.UnescapeString(parsed.Message))
	case "ahelpmessage":
		json_data := []byte(Bquery_deconvert(safe_param(form, "data")))
		var parsed message
		err := json.Unmarshal(json_data, &parsed)
		if err != nil {
			log.Println("json error: ", err)
		}
		if parsed.Ckey != "" && strings.Index(parsed.Ckey, "->") == -1 { //because ADMINPM is AHELP too for some wicked reason
			last_ahelp = parsed.Ckey
		}
		Discord_message_send("admin", "AHELP:", parsed.Ckey, html.UnescapeString(parsed.Message))
	case "memessage":
		json_data := []byte(Bquery_deconvert(safe_param(form, "data")))
		var parsed message
		err := json.Unmarshal(json_data, &parsed)
		if err != nil {
			log.Println("json error: ", err)
		}
		if parsed.Message == "" {
			return //most probably got hit by stunbaton, idk why it sends it
		}
		Discord_message_send("me", "EMOTE:", parsed.Ckey, html.UnescapeString(parsed.Message))
	case "garbage":
		json_data := []byte(Bquery_deconvert(safe_param(form, "data")))
		var parsed message
		err := json.Unmarshal(json_data, &parsed)
		if err != nil {
			log.Println("json error: ", err)
		}
		Discord_message_send("garbage", "", parsed.Ckey, strip.StripTags(html.UnescapeString(parsed.Message)))
	case "token":
		json_data := []byte(Bquery_deconvert(safe_param(form, "data")))
		var parsed token
		err := json.Unmarshal(json_data, &parsed)
		if err != nil {
			log.Println("json error: ", err)
		}
		Discord_process_token(html.UnescapeString(parsed.Token), parsed.Ckey)
	case "roundstatus":
		json_data := []byte(Bquery_deconvert(safe_param(form, "data")))
		var parsed roundstatus
		err := json.Unmarshal(json_data, &parsed)
		if err != nil {
			log.Println("json error: ", err)
		}
		if parsed.Status == "lobby" {
			Discord_subsriber_message_send("bot_status", "new round is about to start (lobby)")
		}
	default:
		log.Print(form)
	}
}

func init() {
	http_webhook_key = os.Getenv("http_webhook_key")
	if http_webhook_key == "" {
		log.Fatalln("Failed to retrieve $http_webhook_key")
	}
	admin_retrieval_page = os.Getenv("admin_retrieval_page")
	if admin_retrieval_page == "" {
		log.Fatalln("Failed to retrieve $admin_retrieval_page")
	}
	port = os.Getenv("PORT")
	if port == "" {
		log.Fatalln("Failed to retrieve $PORT")
	}
}

func Load_admins(str *[]string) {
	*str = nil //clearing
	response, err := http.Get(admin_retrieval_page)
	if err != nil {
		log.Println("FUCK: ", err)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	bodyraw := string(body)
	ind1 := strings.Index(bodyraw, "Architect")
	ind2 := strings.Index(bodyraw, "Removed")
	if ind1 == -1 || ind2 == -1 || ind2 < ind1 {
		log.Println("Fuck")
		return
	}
	bodyraw = bodyraw[ind1+10 : ind2-1]
	for ind3 := strings.Index(bodyraw, "<td>"); ind3 != -1; ind3 = strings.Index(bodyraw, "<td>") {
		ind4 := strings.Index(bodyraw, "</td>")
		*str = append(*str, fmt.Sprint(bodyraw[ind3+4:ind4]))
		bodyraw = bodyraw[ind4+5:]
	}
	bodyraw = string(body)
	ind1 = strings.Index(bodyraw, "Siphon")
	if ind1 == -1 {
		log.Println("Fuck")
		return
	}
	bodyraw = bodyraw[ind1+7:]
	for ind3 := strings.Index(bodyraw, "<td>"); ind3 != -1; ind3 = strings.Index(bodyraw, "<td>") {
		ind4 := strings.Index(bodyraw, "</td>")
		*str = append(*str, fmt.Sprint(bodyraw[ind3+4:ind4]))
		bodyraw = bodyraw[ind4+5:]
	}
	log.Println(*str)
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
