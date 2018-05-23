package main

import (
	"fmt"
	"net/http"
)

const (
	meta_head = `
	<meta name="twitter:card" content="summary"/>
	<meta name="twitter:title" content="%v"/>
	<meta name="twitter:description" content="%v"/>
	<meta name="twitter:image" content="%v"/>
	`
	redirect_head = `
	<script type="text/javascript">
	function redirect() {
		window.location.replace("%v")
	}
	window.onload = redirect
	</script>
	`
	docwrapper = `
	<!DOCTYPE html>
	<html>
	<head>
	<title>regular dank meme</title>
	%v
	</head>
	<body>nothing to see here</body>
	</html>`
)

var meme_stealthmode, meme_first_init_done bool
var (
	meme_meta    string
	meme_content string
)

func meme_init() bool {
	defer logging_recover("epic meme fail")
	title := get_config_must("meme_title")
	desc := get_config_must("meme_description")
	image := get_config_must("meme_image")
	redirect_url := get_config_must("meme_redirect_url")
	meme_meta = fmt.Sprintf(meta_head, title, desc, image)
	meme_content = fmt.Sprintf(redirect_head, redirect_url)
	if !meme_first_init_done {
		meme_first_init_done = true
		http.HandleFunc("/meme", func(w http.ResponseWriter, r *http.Request) {
			if meme_stealthmode {
				meme_stealthmode = false
				fmt.Fprintf(w, docwrapper, meme_meta)
			} else {
				fmt.Fprintf(w, docwrapper, meme_content)
			}
		})
	}
	return true
}

func meme_prime() {
	meme_stealthmode = true
}
