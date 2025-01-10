package utils

import (
	"database/sql"
	"log"
	"os"
)

func DumpSQLiteSchema(db_file string) {
	db, err := sql.Open("sqlite3", db_file)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT sql FROM sqlite_master WHERE type='table'")
	if err != nil {
		log.Fatalf("Failed to query schema: %v", err)
	}
	defer rows.Close()

	file, err := os.Create("schema.txt")
	if err != nil {
		log.Fatalf("Failed to create schema file: %v", err)
	}
	defer file.Close()

	for rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			log.Fatalf("Failed to read schema row: %v", err)
		}
		file.WriteString(sql + "\n")
	}

	log.Println("Schema dumped to schema.txt")
}
