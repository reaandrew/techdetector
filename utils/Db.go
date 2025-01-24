package utils

import (
	"database/sql"
	"encoding/json"
	"fmt"
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

func DumpSQLiteSchemaForFindings(db_file string) {
	db, err := sql.Open("sqlite3", db_file)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	query := "SELECT Properties FROM findings;"
	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("failed to query properties: %w", err)
	}
	defer rows.Close()

	uniqueKeys := make(map[string]bool)

	for rows.Next() {
		var propertiesStr string
		if err := rows.Scan(&propertiesStr); err != nil {
			log.Printf("Failed to scan properties: %v", err)
			continue
		}

		var properties map[string]interface{}
		if err := json.Unmarshal([]byte(propertiesStr), &properties); err != nil {
			log.Printf("Failed to unmarshal JSON properties: %v", err)
			continue
		}

		for key := range properties {
			uniqueKeys[key] = true
		}
	}

	fmt.Println("Available JSON Fields:")
	for key := range uniqueKeys {
		fmt.Println(key)
	}

	log.Println("Schema dumped to schema.txt")
}
