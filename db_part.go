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

func count_query(query string, args ...interface{}) int {
	result, err := Database.Exec(query, args)
	if err != nil {
		log.Println("DB ERROR: failed to count: ", err)
		return -1
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Println("DB ERROR: failed to retrieve amount of rows affected: ", err)
		return -1
	}
	return int(affected)
}
