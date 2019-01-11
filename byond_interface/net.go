package byond_interface

import (
	"fmt"
	"github.com/jammer312/discording/errors"
	"net"
	"time"
)

const (
	ByondTypeNULL   byte = 0x00
	ByondTypeFLOAT  byte = 0x2a
	ByondTypeSTRING byte = 0x06

	//in seconds
	byond_request_timeout      int = 60
	byond_response_timeout     int = 60
	byond_fastrequest_timeout  int = 1
	byond_fastresponse_timeout int = 1
	byond_dial_timeout         int = 1
)

type Byond_response struct {
	size  uint16
	btype byte
	data  []byte
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

func send(addr, request string) Byond_response {
	return send_adv(addr, request, byond_request_timeout, byond_response_timeout)
}

func send_fast(addr, request string) Byond_response {
	return send_adv(addr, request, byond_fastrequest_timeout, byond_fastresponse_timeout)
}

func send_adv(addr, request string, req_to, res_to int) Byond_response {
	defer errors.LogRecover(fmt.Sprintf("query(%v)", addr))
	conn, err := net.DialTimeout("tcp", addr, time.Duration(byond_dial_timeout)*time.Second)
	errors.Deny(err)
	defer conn.Close()

	//sending
	conn.SetWriteDeadline(time.Now().Add(time.Duration(req_to) * time.Second))

	fmt.Fprint(conn, construct_byond_request(request))

	//receiving
	conn.SetReadDeadline(time.Now().Add(time.Duration(res_to) * time.Second))

	bytes := make([]byte, 5)
	num, err := conn.Read(bytes)
	errors.Deny(err)
	L := uint16(bytes[2])<<8 + uint16(bytes[3])
	ret := Byond_response{L - 1, bytes[4], make([]byte, L-1)}
	num, err = conn.Read(ret.data)
	errors.Deny(err)
	errors.Assert(num == int(ret.size), "something is very, very wrong with response (reported size of message differs from actual)")
	return ret
}

func (Br *Byond_response) String() string {
	var ret string
	switch Br.btype {
	case ByondTypeNULL:
		ret = "NULL"
	case ByondTypeFLOAT:
		ret = fmt.Sprintf("%.f", Read_float32(Br.data))
	case ByondTypeSTRING:
		ret = string(Br.data)
		ret = ret[:len(ret)-1]
	}
	return ret
}

func (Br *Byond_response) Float() float32 {
	var ret float32
	if Br.btype == ByondTypeFLOAT {
		ret = Read_float32(Br.data)
	}
	return ret
}
