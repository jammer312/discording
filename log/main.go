package log

import (
	"log"
)

var logger func(line, channel string) error
var filter func(line string) bool //return true if abort logging

func init() {
	filter = func(line string) bool {
		return false
	}
}

func Bind(f func(l, c string) error) {
	logger = f
}

func Unbind() {
	logger = nil
}

func AddFilter(f func(string) bool) {
	tmp := filter //copy it so closure captures value instead of reference
	filter = func(line string) bool {
		return tmp(line) || f(line)
	}
}

func Line(in, ch string) {
	if logger != nil {
		if filter(in) {
			return //filters triggered so abort logging here
		}
		err := logger(in, ch)
		if err != nil {
			log.Println("Bound logger failed: %v", err)
		}
	}
}

func Fatal(in string) {
	Line("**FATAL:** "+in, "runtimes")
	log.Fatal("FATAL: " + in)
}

func LineRuntime(in string) {
	Line(in, "runtimes")
	log.Println(in) // usual logging, 2nd for consistency (see above)
}
