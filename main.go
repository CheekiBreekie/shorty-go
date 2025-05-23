package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

// Define a struct to match the expected POST body
type LinkCreationStruct struct {
	Destination string `json:"destination"`
	Duration    int    `json:"duration_h"`
}

const charset = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNOPQRSTUVWXYZ0123456789" //without I and l

var db *sql.DB

func generateRandomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func indexPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET is allowed", http.StatusMethodNotAllowed)
		return
	}
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	data := struct {
		Title   string
		Heading string
	}{
		Title:   "Index",
		Heading: "Create short link",
	}
	tmpl.Execute(w, data)

}

func handleRedirection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET is allowed", http.StatusMethodNotAllowed)
		return
	}
	vars := mux.Vars(r)
	shortLink := vars["short"]
	fmt.Printf("Incoming request on subpage %s\n", shortLink)
	var destination string
	err := db.QueryRow("SELECT destination FROM records WHERE short_link = ?", shortLink).Scan(&destination)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("Found no matches!\n")
			http.Error(w, "Short link not found", http.StatusNotFound)

			return
		}
		fmt.Printf("Other error: %s\n", err.Error())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !strings.HasPrefix(destination, "http://") && !strings.HasPrefix(destination, "https://") {
		destination = "http://" + destination
	}
	fmt.Printf("Found destination %s\n", destination)
	http.Redirect(w, r, destination, http.StatusFound)

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
	statement, err := db.Prepare("INSERT INTO records(short_link, destination, expiration_date) VALUES (?, ?, ?)")
	if err != nil {
		fmt.Printf("Error preparing db statement: %s", err.Error())
		return
	}
	short_code := generateRandomString(6)
	shorty := fmt.Sprintf("%s/%s", localURL, short_code)
	_, err = statement.Exec(short_code, link.Destination, exp)
	if err != nil {
		fmt.Printf("Error preparing db statement: %s", err.Error())
		return
	}

	response := fmt.Sprintf("Original link=%s\nShort link %s\nExpiration date=%s", link.Destination, shorty, exp)
	fmt.Fprintln(w, response)
}

func hourlyCleanup() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Println("Running database cleanup")
			now := time.Now().Format("2006-01-02 15:04")
			result, err := db.Exec("DELETE FROM records WHERE expiration_date < ?", now)
			if err != nil {
				fmt.Printf("Error during cleanup: %s\n", err.Error())
				continue
			}
			rowsAffected, err := result.RowsAffected()
			if err != nil {
				fmt.Printf("Error during cleanup: %s\n", err.Error())
				continue
			}
			fmt.Printf("Cleaned up %d records\n", rowsAffected)
		}
	}
}

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./database.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	createStatement := `create table if not exists records (
	short_link TEXT PRIMARY KEY NOT NULL,
	destination TEXT NOT NULL,
	creation_date DATETIME DEFAULT CURRENT_TIMESTAMP,
	expiration_date DATETIME)`
	_, err = db.Exec(createStatement)
	if err != nil {
		log.Printf("%q: %s\n", err, createStatement)
		return
	}
	go hourlyCleanup()
	r := mux.NewRouter()
	r.HandleFunc("/index.html", indexPageHandler).Methods("GET")
	r.HandleFunc("/create", handleLinkCreation).Methods("POST", "GET")
	r.HandleFunc("/{short}", handleRedirection).Methods("GET")

	fmt.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
