package errors

import (
	"fmt"
	"github.com/jammer312/discording/log"
	"strings"
)

func LogCrash(ctx string) {
	if r := recover(); r != nil {
		log.Fatal(fmt.Sprintf("[%v]: ", ctx, r))
	}
}

func LogRecover(ctx string) {
	if r := recover(); r != nil {
		log.LineRuntime(fmt.Sprintf("ERR: [%v]: %v", ctx, r))
	}
}

func CallRecover(callback func()) {
	if r := recover(); r != nil {
		callback()
	}
}

func CallRise(callback func()) {
	if r := recover(); r != nil {
		callback()
		panic(r)
	}
}

func Rise(ctx string) {
	if r := recover(); r != nil {
		panic(fmt.Sprintf("[%v]: %v", ctx, r))
	}
}

func Deny(err error) {
	if err != nil {
		panic(err)
	}
}

func Note(err error, supp ...string) {
	if err != nil {
		log.LineRuntime(fmt.Sprintf("**Notice:** %v (`%v`)", err, strings.Join(supp, "`,`")))
	}
}

func Assert(st bool, msg string) {
	if !st {
		panic(msg)
	}
}
