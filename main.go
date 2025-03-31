package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Define a struct to match the expected POST body
type LinkCreationStruct struct {
	Destination string `json:"destination"`
	Duration    int    `json:"duration_h"`
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var db *sql.DB

func generateRandomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func handleLinkCreation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST is allowed", http.StatusMethodNotAllowed)
		return
	}
	host := r.Host // e.g. "localhost:8080" or "example.com"
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	localURL := fmt.Sprintf("%s://%s", scheme, host)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Can't read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var link LinkCreationStruct
	err = json.Unmarshal(body, &link)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Respond with received data
	exp := time.Now().Add(time.Duration(link.Duration * int(time.Hour))).Format("2006-01-02 15:04")
	statement, err := db.Prepare("INSERT INTO records(destination, expiration_date) VALUES (?, ?)")
	if err != nil {
		fmt.Printf("Error preparing db statement: %s", err.Error())
		return
	}
	_, err = statement.Exec(link.Destination, exp)
	if err != nil {
		fmt.Printf("Error preparing db statement: %s", err.Error())
		return
	}
	shorty := fmt.Sprintf("%s/%s", localURL, generateRandomString(6))
	response := fmt.Sprintf("Original link=%s\nShort link %s\nExpiration date=%s", link.Destination, shorty, exp)
	fmt.Fprintln(w, response)
}

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./database.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	createStatement := `create table if not exists records (
	id integer NOT NULL PRIMARY KEY,
	destination TEXT NOT NULL,
	short_link TEXT NOT NULL,
	creation_date DATETIME DEFAULT CURRENT_TIMESTAMP,
	expiration_date DATETIME)`
	_, err = db.Exec(createStatement)
	if err != nil {
		log.Printf("%q: %s\n", err, createStatement)
		return
	}
	http.HandleFunc("/create", handleLinkCreation)

	fmt.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
