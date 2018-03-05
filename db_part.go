package main

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
	"os"
)

var Database *sql.DB

func init() {
	var err error
	Database, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("SQL connect error: ", err)
	}

}
