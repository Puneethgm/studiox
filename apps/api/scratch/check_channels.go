package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("../../.env")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("POSTGRES_DB"),
	)

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	rows, err := pool.Query(context.Background(), "SELECT id, studio_id, kind, external_id, display_handle, status FROM channel_accounts")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("ID | StudioID | Kind | ExternalID | Handle | Status")
	fmt.Println("---|---|---|---|---|---")
	for rows.Next() {
		var id, studioID, kind, externalID, handle, status string
		if err := rows.Scan(&id, &studioID, &kind, &externalID, &handle, &status); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s | %s | %s | %s | %s | %s\n", id, studioID, kind, externalID, handle, status)
	}
}
