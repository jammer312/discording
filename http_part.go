package main

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
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
	port                 string
	admin_retrieval_page string
)

func index_handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "" && r.Method != "GET" {
		return
	} //no POST and other shit
	//no input params, simply prints out various info
	br := Byond_query("white", "status", false)
	out := br.String()
	out = strings.Replace(out, "&", "\n", -1)
	out = strings.Replace(out, "=", ": ", -1)
	out = Bquery_deconvert(out)
	fmt.Fprintln(w, out)
	br = Byond_query("white", "who", false)
	fmt.Fprintln(w, br.String())
}

func safe_param(m *url.Values, param string) string {
	if len((*m)[param]) < 1 {
		return ""
	}
	return (*m)[param][0]
}

type universal_parse struct {
	Ckey     string
	Message  string
	Token    string
	Status   string
	Reason   string
	Seclevel string
}

func webhook_handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "" && r.Method != "GET" {
		return
	} //no POST and other shit
	defer logging_recover("WH")
	r.ParseForm()
	form := &r.Form
	key := safe_param(form, "key")
	var servername string
	for srvname, srv := range known_servers {
		if srv.webhook_key == key {
			servername = srvname
			break
		}
	}
	if servername == "" {
		fmt.Fprint(w, "No command handling without password")
		return
	}
	json_data := []byte(Bquery_deconvert(safe_param(form, "data")))
	var parsed universal_parse
	err := json.Unmarshal(json_data, &parsed)
	if err != nil {
		panic(err)
	}
	switch safe_param(form, "method") {
	case "oocmessage":
		Discord_message_send(servername, "ooc", "OOC:", parsed.Ckey, html.UnescapeString(parsed.Message))
	case "asaymessage":
		Discord_message_send(servername, "admin", "ASAY:", parsed.Ckey, html.UnescapeString(parsed.Message))
	case "ahelpmessage":
		if parsed.Ckey != "" && strings.Index(parsed.Ckey, "->") == -1 { //because ADMINPM is AHELP too for some wicked reason
			last_ahelp[servername] = parsed.Ckey
		}
		Discord_message_send(servername, "admin", "AHELP:", parsed.Ckey, html.UnescapeString(parsed.Message))
	case "memessage":
		if parsed.Message == "" {
			return //probably got hit by stunbaton, idk why it sends it
		}
		Discord_message_send(servername, "me", "EMOTE:", parsed.Ckey, html.UnescapeString(parsed.Message))
	case "garbage":
		Discord_message_send(servername, "garbage", "", parsed.Ckey, strip.StripTags(html.UnescapeString(parsed.Message)))
	case "token":
		Discord_process_token(html.UnescapeString(parsed.Token), parsed.Ckey)
	case "runtimemessage":
		Discord_message_send(servername, "debug", "DEBUG:", "RUNTIME", html.UnescapeString(parsed.Message))
	case "roundstatus":
		/*color := 0
		if servername != "" {
			color = known_servers[servername].color
		}*/
		embed := &discordgo.MessageEmbed{
			Author:      &discordgo.MessageEmbedAuthor{},
			Color:       0x00ff00, // Green
			Title:       "I am an Embed",
			Description: "",
			Fields:      []*discordgo.MessageEmbedField{},
		}
		out, _ := json.Marshal(&discordgo.MessageSend{
			Embed: embed,
		})
		fmt.Println(string(out))
		Discord_send_embed(servername, "debug", embed)
		switch parsed.Status {
		case "lobby":
			Discord_subsriber_message_send(servername, "bot_status", "New round is about to start (lobby)")

		case "shuttle called":
			Discord_message_send(servername, "ooc", "", "ROUND STATUS", "Shuttle called")

		case "shuttle recalled":
			Discord_message_send(servername, "ooc", "", "ROUND STATUS", "Shuttle recalled")

		case "shuttle autocalled":
			Discord_message_send(servername, "ooc", "", "ROUND STATUS", "Shuttle autocalled")

		case "shuttle docked":
			Discord_message_send(servername, "ooc", "", "ROUND STATUS", "Shuttle docked with the station")

		case "shuttle left":
			Discord_message_send(servername, "ooc", "", "ROUND STATUS", "Shuttle left the station")

		case "shuttle escaped":
			Discord_message_send(servername, "ooc", "", "ROUND STATUS", "Shuttle docked with centcomm")
			Discord_subsriber_message_send(servername, "bot_status", "Current round is about to end (roundend)")

		case "reboot":
			Discord_message_send_raw(servername, "ooc", "**===REBOOT===**")

		}
	default:
		log.Print(form)
	}
}

func init() {
	port = os.Getenv("PORT")
	if port == "" {
		log.Fatalln("Failed to retrieve $PORT")
	}
}

func Load_admins() {
	for s := range known_servers {
		Load_admins_for_server(s)
	}
}

func Load_admins_for_server(server string) {
	logging_recover("ADM " + server)
	servstruct, ok := known_servers[server]
	if !ok {
		panic("can't find server")
	}
	response, err := http.Get(servstruct.admins_page)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	bodyraw := string(body)

	admins := make(map[string][]string)
	if err := json.Unmarshal([]byte(bodyraw), &admins); err != nil {
		panic(err)
	}
	adminssl := make([]string, 0)
	for k, v := range admins {
		if k == "Removed" {
			continue
		}
		adminssl = append(adminssl, v...)
	}
	Known_admins[server] = adminssl
	log.Println(adminssl)
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
