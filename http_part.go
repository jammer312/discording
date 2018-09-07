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
	"strconv"
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
	Ckey         string
	Message      string
	Token        string
	Status       string
	Reason       string
	Seclevel     string
	Event        string
	Data         string
	Round        string
	Keyname      string
	Role         string
	Add_num      string
	Has_follower string
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
		Discord_message_send(servername, "debug", "DEBUG", "", html.UnescapeString(parsed.Message))
	case "roundstatus":
		color := known_servers[servername].color
		embed := &discordgo.MessageEmbed{
			Color:  color,
			Fields: []*discordgo.MessageEmbedField{},
		}
		ss, ok := server_statuses[servername]
		ss_glob_update := func() {
			if ok {
				ss.global_update()
			}
		}
		switch parsed.Status {
		case "lobby":
			Discord_subsriber_message_send(servername, "bot_status", "New round is about to start (lobby)")
			ss_glob_update()

		case "ingame":
			Discord_subsriber_message_send(servername, "bot_status", "New round had just started")
			ss_glob_update()

		case "shuttle called":
			embed.Fields = []*discordgo.MessageEmbedField{&discordgo.MessageEmbedField{Name: "Code:", Value: parsed.Seclevel, Inline: true}, &discordgo.MessageEmbedField{Name: "Reason:", Value: Dsanitize(parsed.Reason), Inline: true}}
			embed.Title = "SHUTTLE CALLED"
			Discord_send_embed(servername, "bot_status", embed)
			Discord_send_embed(servername, "ooc", embed)
			ss_glob_update()

		case "shuttle recalled":
			embed.Title = "SHUTTLE RECALLED"
			Discord_send_embed(servername, "bot_status", embed)
			Discord_send_embed(servername, "ooc", embed)
			ss_glob_update()

		case "shuttle autocalled":
			embed.Title = "SHUTTLE AUTOCALLED"
			Discord_send_embed(servername, "bot_status", embed)
			Discord_send_embed(servername, "ooc", embed)
			ss_glob_update()

		case "shuttle docked":
			embed.Title = "SHUTTLE DOCKED WITH THE STATION"
			Discord_send_embed(servername, "bot_status", embed)
			Discord_send_embed(servername, "ooc", embed)
			ss_glob_update()

		case "shuttle left":
			embed.Title = "SHUTTLE LEFT THE STATION"
			Discord_send_embed(servername, "bot_status", embed)
			Discord_send_embed(servername, "ooc", embed)
			ss_glob_update()

		case "shuttle escaped":
			embed.Title = "SHUTTLE DOCKED WITH CENTCOMM"
			Discord_send_embed(servername, "bot_status", embed)
			Discord_send_embed(servername, "ooc", embed)
			Discord_subsriber_message_send(servername, "bot_status", "Current round is about to end (roundend)")
			ss_glob_update()

		case "reboot":
			Discord_message_send_raw(servername, "ooc", "**===REBOOT===**")

		}
	case "status_update":
		ss, ok := server_statuses[servername]
		if !ok {
			return
		}
		switch parsed.Event {
		case "client_login", "client_logoff":
			ss.global_update()

		default:
			log.Print(form)
		}
	case "data_request":
		if parsed.Data == "shitspawn_list" {
			//CKEYS (ckey_simplifier)
			round, err := strconv.Atoi(parsed.Round)
			noerror(err)
			str := check_donators(servername, round)
			fmt.Fprint(w, str)
			log_line("shitspawn list -> "+str, "shitspawn_debug")
		}
	case "rolespawn":
		round, err := strconv.Atoi(parsed.Round)
		noerror(err)
		add_num, err := strconv.Atoi(parsed.Add_num)
		noerror(err)
		has_follower, err := strconv.Atoi(parsed.Has_follower)
		noerror(err)
		expend_donator(servername, parsed.Keyname, round, parsed.Role, add_num, has_follower > 0)
		log_line(fmt.Sprintf("shitspawn role -> %v %v %v %v %v %v", servername, parsed.Keyname, parsed.Round, parsed.Role, parsed.Add_num, parsed.Has_follower), "shitspawn_debug")
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
	defer logging_recover("ADM " + server)
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
