package utils

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
)

func DumpSQLiteSchemaForFindings(db_file string) {
	db, err := sql.Open("sqlite3", db_file)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	query := "SELECT Properties FROM findings;"
	rows, err := db.Query(query)

	if err != nil {
		log.Fatalf("failed to query properties: %v", err)
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
