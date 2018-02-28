package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"golang.org/x/text/encoding/charmap"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	ByondTypeNULL   byte = 0x00
	ByondTypeFLOAT  byte = 0x2a
	ByondTypeSTRING byte = 0x06

	byond_request_timeout  int = 5 //in seconds
	byond_response_timeout int = 6 //in seconds
)

var (
	byond_server_addr string = "placeholder"
	byond_pass_key    string = "placeholder"

	discord_bot_token string = "placeholder"
)

type Byond_response struct {
	size  uint16
	btype byte
	data  []byte
}

func DecodeWindows1251(s string) string {
	dec := charmap.Windows1251.NewDecoder()
	out, _ := dec.String(s)
	return out
}

func EncodeWindows1251(s string) string {
	enc := charmap.Windows1251.NewEncoder()
	out, _ := enc.String(s)
	return out
}

func Read_float32(data []byte) (ret float32) {
	buf := bytes.NewBuffer(data)
	binary.Read(buf, binary.LittleEndian, &ret)
	return
}

func construct_byond_request(s string) string {
	var B uint16 = uint16(len(s) + 6)
	var bytes []byte
	bytes = append(bytes, 0x00, 0x83, byte(B>>8), byte((B<<8)>>8), 0x00, 0x00, 0x00, 0x00, 0x00)
	bytes = append(bytes, []byte(s)...)
	bytes = append(bytes, 0x00)
	ret := string(bytes)
	return ret
}

func Byond_query(request string) Byond_response {
	conn, err := net.Dial("tcp", byond_server_addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	//sending
	conn.SetWriteDeadline(time.Now().Add(time.Duration(byond_request_timeout) * time.Second))

	fmt.Fprint(conn, construct_byond_request(request))

	//receiving
	conn.SetReadDeadline(time.Now().Add(time.Duration(byond_response_timeout) * time.Second))
	bytes := make([]byte, 5)
	num, err := conn.Read(bytes)
	if err != nil {
		log.Fatal("Reading error: ", err)
	}
	L := uint16(bytes[2])<<8 + uint16(bytes[3])
	ret := Byond_response{L - 1, bytes[4], make([]byte, L-1)}
	num, err = conn.Read(ret.data)
	if err != nil {
		log.Fatal("Data reading error: ", err)
	}
	if num != int(ret.size) {
		log.Fatal("Shit happened")
	}
	return ret
}

func Escape_and_encode(s string) string {
	return url.QueryEscape(EncodeWindows1251(s))
}

func main() {
	rainbowcolors := []string{"ff0000", "ff7f00", "ffff00", "00ff00", "0000ff", "4B0082", "9400D3"}
	reader := bufio.NewReader(os.Stdin)
	i := 0
	for {
		message, err := reader.ReadString('\n')
		message = strings.TrimRight(message, "\n")
		if err != nil {
			log.Fatal(err)
		}
		Byond_query("announce=" + Escape_and_encode(message) + "&i=" + rainbowcolors[i] + "&g=Jammer312")
		i = (i + 1) % 7
	}
}
