package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

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

	studioID := "ae73bd74-417a-4351-9ecc-4b610ce474c7"
	rows, err := pool.Query(context.Background(), `
		SELECT c.id, ch.kind, c.external_thread_id, c.last_message_preview, c.created_at
		FROM conversations c
		JOIN channel_accounts ch ON ch.id = c.channel_account_id
		WHERE c.studio_id = $1
		ORDER BY c.created_at DESC
	`, studioID)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("ID | Kind | ExtThreadID | Preview | CreatedAt")
	fmt.Println("---|---|---|---|---")
	for rows.Next() {
		var id, kind, extID, preview string
		var createdAt time.Time
		if err := rows.Scan(&id, &kind, &extID, &preview, &createdAt); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s | %s | %s | %s | %s\n", id, kind, extID, preview, createdAt.Format(time.RFC3339))
	}
}
