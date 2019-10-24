package main

import (
	"database/sql"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func DBconnection() (db *sql.DB) {
	dbDriver := "mysql"
	dbUser := os.Getenv("MYSQL_USER")
	dbPassword := os.Getenv("MYSQL_PASSWORD")
	dbName := os.Getenv("MYSQL_DATABASE")

	// when using containers - no matter what port mysql is mapped to it uses only service name to depend on...
	//db, err := sql.Open(dbDriver, dbUser+":"+dbPassword+"@tcp(mysqlserver)/"+dbName)
	// to test in dev environment
	db, err := sql.Open(dbDriver, dbUser+":"+dbPassword+"@tcp(127.0.0.1:3350)/"+dbName+"?parseTime=true")
	checkErr(err)

	return db
}
