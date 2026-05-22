package main

import (
	"context"
	"fmt"
	"os"

	"github.com/projectx/api/internal/platform/config"
	"github.com/projectx/api/internal/platform/db"
	"github.com/projectx/api/internal/studios"
)

func main() {
	cfg, _ := config.Load()
	pool, _ := db.Connect(context.Background(), cfg.DB.DSN())
	repo := studios.NewRepo(pool)
	_, err := repo.List(context.Background())
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	fmt.Println("Success")
}
