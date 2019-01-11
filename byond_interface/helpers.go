package byond_interface

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/jammer312/discording/errors"
	"github.com/jammer312/discording/log"
	"golang.org/x/text/encoding/charmap"
	"net/url"
	"regexp"
)

func DecodeWindows1251(s string) string {
	dec := charmap.Windows1251.NewDecoder()
	out, err := dec.String(s)
	errors.Note(err, s)
	return out
}

func EncodeWindows1251(s string) string {
	enc := charmap.Windows1251.NewEncoder()
	out, err := enc.String(s)
	errors.Note(err, s)
	return out
}

func Bquery_convert(s string) string {
	return url.QueryEscape(EncodeWindows1251(s))
}

func Bquery_deconvert(s string) string {
	ret, err := url.QueryUnescape(s)
	if err != nil {
		log.LineRuntime(fmt.Sprintf("ERROR: Query unescape error: %v", err))
		return ret
	}
	ret = DecodeWindows1251(ret)
	return ret
}

var json_fixer *regexp.Regexp

func Fix_byond_json(s string) string {
	if json_fixer == nil {
		json_fixer = regexp.MustCompile("\\\\u044f[\\x12\\x16]")
	}
	return json_fixer.ReplaceAllString(s, "")
}

func Read_float32(data []byte) (ret float32) {
	buf := bytes.NewBuffer(data)
	binary.Read(buf, binary.LittleEndian, &ret)
	return
}
