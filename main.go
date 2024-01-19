package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"text/template"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

var tmpl template.Template = *template.Must(template.ParseFiles("./templates/index.html"))
var db *sql.DB

func main() {
    var err error
	db, err = sql.Open("sqlite3", "webshelf.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/search/", searchHandler).Methods("GET")

	http.Handle("/", r)

	log.Println("App running on localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "index", nil)
}

type Book struct {
	Name           string
	Url            string
	CurrentChapter int
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query_string := "select name, url, currentChapter from books where name like ?"
	query_key := fmt.Sprintf("%%%s%%", r.URL.Query().Get("key"))

	resultRows, err := db.Query(query_string, query_key)
	if err != nil {
		log.Println("db query error")
		log.Fatal(err)
	}

	var searchResults []Book
	for resultRows.Next() {
		var name string
		var url string
		var currentChapter int

		err = resultRows.Scan(&name, &url, &currentChapter)
		if err != nil {
			log.Fatal(err)
		}

		book := Book{Name: name, Url: url, CurrentChapter: currentChapter}
		searchResults = append(searchResults, book)
	}

	data := map[string][]Book{
		"Results": searchResults,
	}
	tmpl.ExecuteTemplate(w, "index", data)
}

func addBook(w http.ResponseWriter, r *http.Request) {
	defer db.Close()
}
