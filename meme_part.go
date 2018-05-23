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
	docwrapper = `
	<!DOCTYPE html>
	<html>
	<head>
	<title>regular dank meme</title>
	%v
	</head>
	<body><iframe width="854" height="480" src="https://www.youtube.com/embed/dQw4w9WgXcQ?autoplay=1" frameborder="0" allow="autoplay; encrypted-media" style="margin-left: auto; margin-right: auto;" allowfullscreen></iframe></body>
	</html>`
)

var meme_first_init_done bool
var (
	meme_meta    string
	meme_content string
)

func meme_init() bool {
	defer logging_recover("epic meme fail")
	title := get_config_must("meme_title")
	desc := get_config_must("meme_description")
	image := get_config_must("meme_image")
	meme_meta = fmt.Sprintf(meta_head, title, desc, image)
	if !meme_first_init_done {
		meme_first_init_done = true
		http.HandleFunc("/meme", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, docwrapper, meme_meta)
		})
	}
	return true
}
