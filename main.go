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
	r.HandleFunc("/book/", addBook).Methods("POST")
	r.HandleFunc("/book/{id:[0-9]+}/", editBook).Methods("PATCH")
	r.HandleFunc("/book/{id:[0-9]+}/edit", editHandler).Methods("GET")
	r.HandleFunc("/book/{id:[0-9]+}/", deleteBook).Methods("DELETE")

	http.Handle("/", r)

	log.Println("App running on localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	query_string := "select id, name, url, currentChapter from books" 
	resultRows, err := db.Query(query_string)
	if err != nil {
		log.Println("db query error")
		log.Fatal(err)
	}

	var searchResults []Book
	for resultRows.Next() {
        var id string
		var name string
		var url string
		var currentChapter string

		err = resultRows.Scan(&id, &name, &url, &currentChapter)
		if err != nil {
			log.Fatal(err)
		}

        book := Book{Id: id, Name: name, Url: url, CurrentChapter: currentChapter}
		searchResults = append(searchResults, book)
	}

	data := map[string][]Book{
		"Results": searchResults,
	}

    // bit inefficient, should look at how to make this not 
    // send the entire thing when redirecting
	tmpl.ExecuteTemplate(w, "index", data)
}

type Book struct {
    Id string
	Name           string
	Url            string
	CurrentChapter string
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query_string := "select id, name, url, currentChapter from books where name like ?"
	query_key := fmt.Sprintf("%%%s%%", r.URL.Query().Get("key"))

	resultRows, err := db.Query(query_string, query_key)
	if err != nil {
		log.Println("db query error")
		log.Fatal(err)
	}

	var searchResults []Book
	for resultRows.Next() {
        var id string
		var name string
		var url string
		var currentChapter string

		err = resultRows.Scan(&id, &name, &url, &currentChapter)
		if err != nil {
			log.Fatal(err)
		}

        book := Book{Id: id, Name: name, Url: url, CurrentChapter: currentChapter}
		searchResults = append(searchResults, book)
	}

	data := map[string][]Book{
		"Results": searchResults,
	}
	tmpl.ExecuteTemplate(w, "bookList", data)
}

func addBook(w http.ResponseWriter, r *http.Request) {
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	statement, err := tx.Prepare("insert into books(name, url, currentChapter) values(?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer statement.Close()

	newBook := Book{
		Name:           r.FormValue("bookName"),
		Url:            r.FormValue("bookUrl"),
		CurrentChapter: r.FormValue("bookChapter"),
	}
	_, err = statement.Exec(newBook.Name, newBook.Url, newBook.CurrentChapter)
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}

    // might need to redirect this
    http.Redirect(w, r, "/", http.StatusSeeOther)
}

func deleteBook(w http.ResponseWriter, r *http.Request) {
    tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	statement, err := tx.Prepare("delete from books where id = ?")
	if err != nil {
		log.Fatal(err)
	}
    vars := mux.Vars(r)
	_, err = statement.Exec(vars["id"]) 
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}

    //TODO also redirect here
	tmpl.ExecuteTemplate(w, "index", nil)
}

func editBook(w http.ResponseWriter, r *http.Request) {
}


func editHandler(w http.ResponseWriter, r *http.Request) {
}
