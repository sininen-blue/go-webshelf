package main

import (
	"database/sql"
	"log"
	"net/http"
	nurl "net/url"
	"strconv"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

var tmpl template.Template = *template.Must(template.ParseFiles("./templates/index.html", "./templates/history.html"))
var db *sql.DB

const timeLayout string = "2006/01/02 15:04:05"

var trimColor = map[string]string{
	"archiveofourown.org":           "red",
	"www.royalroad.com":             "amber",
	"www.fanfiction.net":            "blue",
	"forums.sufficientvelocity.com": "cyan",
}

func init() {
	var err error
	db, err = sql.Open("sqlite3", "webshelf.db")
	if err != nil {
		log.Fatal(err)
	}

	sqlSchema := `
    CREATE TABLE if not exists "books" (
        "id"	INTEGER,
        "name"	TEXT,
        "url"	TEXT,
        "currentChapter"	TEXT,
        "dateCreated"	TEXT,
        "dateUpdated"	INTEGER,
        PRIMARY KEY("id")
    );
    CREATE TABLE if not exists "history" (
        "id"	INTEGER,
        "date"	TEXT,
        "action"	TEXT,
        PRIMARY KEY("id")
    );
    `
	_, err = db.Exec(sqlSchema)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	defer db.Close()

	r := mux.NewRouter()
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/updates/", updateHandler).Methods("GET")
	r.HandleFunc("/updates/full", updateListHandler).Methods("GET")
	r.HandleFunc("/search/", searchHandler).Methods("GET")
	r.HandleFunc("/book/", addBook).Methods("POST")
	r.HandleFunc("/book/{id:[0-9]+}/", editBook).Methods("PATCH")
	r.HandleFunc("/book/{id:[0-9]+}/edit", editHandler).Methods("GET")
	r.HandleFunc("/book/{id:[0-9]+}/", deleteBook).Methods("DELETE")

	http.Handle("/", r)

	log.Println("App running on localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func dbGetBooks(columns string, condition string, key string) []Book {
	var err error
	var rows *sql.Rows

	if condition == "" {
		query_string := "select " + columns + " from books order by dateUpdated desc"
		rows, err = db.Query(query_string)
	} else {
		query_string := "select " + columns + " from books where " + condition + " order by dateUpdated desc"
		rows, err = db.Query(query_string, key)
	}
	if err != nil {
		log.Println("db query error")
		log.Fatal(err)
	}

	var results []Book
	for rows.Next() {
		var id string
		var name string
		var url string
		var currentChapter string
		var dateCreated string
		var dateUpdated string

		err = rows.Scan(&id, &name, &url, &currentChapter, &dateCreated, &dateUpdated)
		if err != nil {
			log.Fatal(err)
		}

		parsedUrl, _ := nurl.Parse(url)
		if err != nil {
			log.Fatal(err)
		}
		color := trimColor[parsedUrl.Host]
		if color == "" {
			color = "slate"
		}

		book := Book{
			Id:             id,
			Name:           name,
			Url:            url,
			CurrentChapter: currentChapter,
			DateCreated:    dateCreated,
			DateUpdated:    dateUpdated,
			Color:          color,
		}
		results = append(results, book)
	}

	return results
}

func getPagedBooks(page int) []Book {
	var bookList []Book

	rows, err := db.Query("select * from books order by dateUpdated desc limit 10 offset ?", (page-1) * 10)
	if err != nil {
		log.Fatal(err)
	}
    
    for rows.Next() {
        var b Book
        err = rows.Scan(&b.Id, &b.Name, &b.Url, &b.CurrentChapter, &b.DateCreated, &b.DateUpdated) 
        if err != nil {
            log.Fatal(err)
        }

		parsedUrl, _ := nurl.Parse(b.Url)
		color := trimColor[parsedUrl.Host]
		if color == "" {
			color = "slate"
		}
        b.Color = color

        bookList = append(bookList, b)
    }

	return bookList
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
    page := r.URL.Query().Get("page")
    if page == "" {
        page = "1"
    }
    pageInt, err := strconv.Atoi(page)
    if err != nil {
        log.Fatal(err)
    }

    searchResults := getPagedBooks(pageInt)
	// second query

	query_string := "select date, action from history order by date desc limit 5 "

	resultRows, err := db.Query(query_string)
	if err != nil {
		log.Println("I hate strings so much")
		log.Fatal(err)
	}

	var recentHistory []History
	for resultRows.Next() {
		var date string
		var action string

		err = resultRows.Scan(&date, &action)
		if err != nil {
			log.Fatal(err)
		}

		item := History{
			Date:   date,
			Action: action,
		}

		recentHistory = append(recentHistory, item)
	}

	data := map[string]interface{}{
        "Page": pageInt,
		"Results": searchResults,
		"Updates": recentHistory,
	}

	// bit inefficient, should look at how to make this not
	// send the entire thing when redirecting
	w.Header().Add("HX-TRIGGER", "newAction")
	tmpl.ExecuteTemplate(w, "index", data)
}

type History struct {
	Date   string
	Action string
}

type Book struct {
	Id             string
	Name           string
	Url            string
	Color          string
	CurrentChapter string
	DateCreated    string
	DateUpdated    string
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	query_key := "%" + r.URL.Query().Get("q") + "%"
	searchResults := dbGetBooks("*", "name like ?", query_key)

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
	statement, err := tx.Prepare("insert into books(name, url, currentChapter, dateCreated, dateUpdated) values(?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer statement.Close()

	currentTime := time.Now().Format(timeLayout)
	newBook := Book{
		Name:           r.FormValue("bookName"),
		Url:            r.FormValue("bookUrl"),
		CurrentChapter: r.FormValue("bookChapter"),
		DateCreated:    currentTime,
		DateUpdated:    currentTime,
	}
	addResult, err := statement.Exec(newBook.Name, newBook.Url, newBook.CurrentChapter, newBook.DateCreated, newBook.DateUpdated)
	if err != nil {
		log.Fatal(err)
	}

	historyStatement, err := tx.Prepare("insert into history(date, action) values(?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	_, err = historyStatement.Exec(currentTime, "added "+r.FormValue("bookName"))
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}

	var book Book
	bookId, err := addResult.LastInsertId()
	if err != nil {
		log.Fatal("please")
		log.Fatal(err)
	}
	bookRow := db.QueryRow("select * from books where id == ?", bookId)
	err = bookRow.Scan(&book.Id, &book.Name, &book.Url, &book.CurrentChapter, &book.DateCreated, &book.DateUpdated)
	if err != nil {
		log.Fatal(err)
	}

	book.Color = "green"

	w.Header().Add("HX-TRIGGER", "newAction")
	tmpl.ExecuteTemplate(w, "book", book)
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

	var name string
	resultRow := db.QueryRow("select name from books where id = ?", vars["id"])
	err = resultRow.Scan(&name)
	if err != nil {
		log.Fatal(err)
	}

	currentTime := time.Now().Format(timeLayout)
	historyStatement, err := tx.Prepare("insert into history(date, action) values(?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	_, err = historyStatement.Exec(currentTime, "deleted "+name)
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Add("HX-TRIGGER", "newAction")
}

func editBook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	var currentBook Book
	bookId := vars["id"]
	bookRow := db.QueryRow("select * from books where id = ?", bookId)
	err := bookRow.Scan(&currentBook.Id, &currentBook.Name, &currentBook.Url, &currentBook.CurrentChapter, &currentBook.DateCreated, &currentBook.DateUpdated)
	if err != nil {
		log.Fatal(err)
	}

	editUrl := r.FormValue("bookUrl")
	editName := r.FormValue("bookName")
	editCurrentChapter := r.FormValue("bookChapter")

	currentTime := time.Now().Format(timeLayout)

	if currentBook.Url == editUrl && currentBook.Name == editName && currentBook.CurrentChapter == editCurrentChapter {
		log.Println("no changes")
	} else {
		tx, err := db.Begin()
		if err != nil {
			log.Fatal(err)
		}
		statement, err := tx.Prepare("update books set url = ?, name = ?, currentChapter = ?, dateUpdated = ? where id = ?")
		if err != nil {
			log.Fatal(err)
		}

		url := r.FormValue("bookUrl")
		name := r.FormValue("bookName")
		currentChapter := r.FormValue("bookChapter")
		dateUpdated := currentTime

		_, err = statement.Exec(url, name, currentChapter, dateUpdated, vars["id"])
		if err != nil {
			log.Fatal(err)
		}

		// update item queries
		historyStatement, err := tx.Prepare("insert into history(date, action) values(?, ?)")
		if err != nil {
			log.Fatal(err)
		}

		_, err = historyStatement.Exec(currentTime, "edited "+name)
		if err != nil {
			log.Fatal(err)
		}

		err = tx.Commit()
		if err != nil {
			log.Fatal(err)
		}
	}

	w.Header().Add("HX-TRIGGER", "newAction")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	query_string := "select id, name, url, currentChapter from books where id = ?"
	vars := mux.Vars(r)

	resultRow := db.QueryRow(query_string, vars["id"])

	var id string
	var name string
	var url string
	var currentChapter string

	err := resultRow.Scan(&id, &name, &url, &currentChapter)
	if err != nil {
		log.Fatal(err)
	}

	book := Book{Id: id, Name: name, Url: url, CurrentChapter: currentChapter}

	tmpl.ExecuteTemplate(w, "bookEdit", book)
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	query_string := "select date, action from history order by date desc limit 5 "

	resultRows, err := db.Query(query_string)
	if err != nil {
		log.Println("I hate strings so much")
		log.Fatal(err)
	}

	var recentHistory []History
	for resultRows.Next() {
		var date string
		var action string

		err = resultRows.Scan(&date, &action)
		if err != nil {
			log.Fatal(err)
		}

		item := History{
			Date:   date,
			Action: action,
		}

		recentHistory = append(recentHistory, item)
	}

	data := map[string][]History{
		"Updates": recentHistory,
	}
	tmpl.ExecuteTemplate(w, "recentUpdates", data)
}

func updateListHandler(w http.ResponseWriter, r *http.Request) {
	query_string := "select date, action from history order by date desc"

	resultRows, err := db.Query(query_string)
	if err != nil {
		log.Println("I hate strings so much")
		log.Fatal(err)
	}

	var recentHistory []History
	for resultRows.Next() {
		var date string
		var action string

		err = resultRows.Scan(&date, &action)
		if err != nil {
			log.Fatal(err)
		}

		item := History{
			Date:   date,
			Action: action,
		}

		recentHistory = append(recentHistory, item)
	}

	data := map[string][]History{
		"Updates": recentHistory,
	}

	tmpl.ExecuteTemplate(w, "allUpdates", data)
}
