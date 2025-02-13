package db

import (
	"database/sql"
	"fmt"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"os"

	"strconv"
)

var Db *sql.DB

func ConnectDatabase() {
	err := godotenv.Load()

	if err != nil {
		fmt.Println("Failed load .env file")
	}

	host := os.Getenv("DB_HOST")
	port, _ := strconv.Atoi(os.Getenv("DB_PORT"))
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	psqlSetup := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbName)
	db, sqlErr := sql.Open("postgres", psqlSetup)
	
	if sqlErr != nil {
		fmt.Println("failed connecting to database", err)
		panic(err)
	} else {
		Db = db
		fmt.Println("success connecting to database")
	}
}