package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
)

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://projectx:projectx_dev@localhost:5435/projectx")
	if err != nil {
		log.Fatal(err)
	}
	var val string
	err = pool.QueryRow(context.Background(), "SELECT value FROM platform_settings WHERE key = $1", "stripe_secret_key").Scan(&val)
	fmt.Printf("Key from DB: %q, Error: %v\n", val, err)
}
