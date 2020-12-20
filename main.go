package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"net/http"
	"os"
	"strings"
	"bytes"
)

// config
const (
	DIR = ".dictionarylogger"
	DB  = "dictionarylogger.db"
)

// permissions
const (
	PERMISSIONS = 0755 // rwx, only owner can write and execute
)

// commands
const (
	SEARCH = "search"
	LIST   = "list"
	HELP   = "help"
)

// flags
const (
	ONELINE = "--oneline"
)

type SearchResult struct {
	Meanings []struct {
		PartOfSpeech string `json:"partOfSpeech"`
		Definitions  []struct {
			Definition string `json:"definition"`
			Example    string `json:"example"`
		} `json:"definitions"`
	} `json: "meanings"`
}

func (sr SearchResult) String() string {
	
	var buffer bytes.Buffer
	

	for i := 0; i < len(sr.Meanings); i++ {
		buffer.WriteString(fmt.Sprintf("%s\n", sr.Meanings[i].PartOfSpeech))
		for j := 0; j < len(sr.Meanings[i].Definitions); j++ {
			buffer.WriteString(fmt.Sprintf("\t\tDefinition: \n \t\t\t\t %s\n", 
				sr.Meanings[i].Definitions[j].Definition))
			buffer.WriteString(fmt.Sprintf("\t\tExample: \n \t\t\t\t %s\n\n", 
				sr.Meanings[i].Definitions[j].Example))
		} 
	}

	return buffer.String()
}

type SearchResults []SearchResult

func (srs SearchResults) String() string {
	var strBuff bytes.Buffer

	for i := 0; i < len(srs); i++ {
		strBuff.WriteString(fmt.Sprintf("Definition %d: \n\n", i))
		strBuff.WriteString(fmt.Sprintf("\t%s", srs[i]))
	}

	return strBuff.String()
}

var db *sql.DB

func initDB() {

	createDefinitionTableSQL := `CREATE TABLE definition (
		word TEXT,
		results TEXT
	); ALTER TABLE definition ADD CONSTRAINT UQ_Word UNIQUE (word);`

	statement, err := db.Prepare(createDefinitionTableSQL)
	if err != nil {
		return
	}

	_, err = statement.Exec()

	if err != nil {
		fmt.Println("unable to initialise DB")
		os.Exit(1)
	}

	return

}

func addResult(word string, res string) (err error) {

	insertResultSQL := fmt.Sprintf(`INSERT INTO definition (word, results) VALUES ("%s", "%s");`, word, res)

	statement, err := db.Prepare(insertResultSQL)
	if err != nil {
		return
	}

	_, err = statement.Exec()

	return
}

func queryResultsByWord(word string) (sr string, err error) {

	queryByWordSQL := fmt.Sprintf(`SELECT results FROM definition WHERE word="%s";`, word)

	row, err := db.Query(queryByWordSQL)
	if err != nil {
		return 
	}

	if row.Next() {
		row.Scan(&sr)
	}

	return

}

func retrieveAllResults() (rs map[string]string, err error) {
	rs = map[string]string{}

	selectAllQuerySQL := `SELECT word, results from definition;`

	row, err := db.Query(selectAllQuerySQL)
	if err != nil {
		return 
	}
	

	for row.Next() {
		var word string
		var results string 
		row.Scan(&word,&results)
		rs[word] = results
	}

	return 
}

func init() {

	dirPath := fmt.Sprintf("%s/%s", os.Getenv("HOME"), DIR)
	dbPath := fmt.Sprintf("%s/%s", dirPath, DB)

	// create dir if uninitialised
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			// create home dir
			if err := os.MkdirAll(dirPath, PERMISSIONS); err != nil {
				fmt.Printf("unable to create internal directories | %s\n", err.Error())
				os.Exit(1)
			}
			// sqlite db
			if _, err := os.Create(dbPath); err != nil {
				fmt.Printf("unable to create database | %s\n", err)
				os.Exit(1)
			} else {
				defer initDB()
			}

		} else {
			fmt.Println("unable to initialise")
			os.Exit(1)
		}
	}

	// used to store new results and request old results
	var err error
	if db, err = sql.Open("sqlite3", dbPath); err != nil {
		fmt.Printf("unable to open database | %s\n", err)
		os.Exit(1)
	}
}

func help() {

	if len(os.Args) != 2 {
		fmt.Println("no arguments required for help command")
		os.Exit(0)
	}

	fmt.Printf(`
	%s: List all commands

	%s: Print all previous searches, --oneline to omit definitions

	%s: Search for definition and record result
	
`,
		HELP, LIST, SEARCH)

}

func list() {

	oneline := false
	if len(os.Args) > 2 {
		if os.Args[2] == ONELINE {
			oneline = true
		}
	}

	history, err := retrieveAllResults()
	if err != nil {
		fmt.Printf("unable to fetch all results | %s\n", err.Error())
		os.Exit(1)
	}

	for word, result := range history {
		fmt.Printf("Word: %s\n", word);
		if !oneline {
			fmt.Println(result)
		}
	}
}

func search() {

	if len(os.Args) < 3 {
		fmt.Println("requires a single word after the search command")
		return
	}

	word := strings.ToLower(os.Args[2])
	
	var result string
	var err error 
	if result, err = queryResultsByWord(word); err != nil || result == "" {

		// google dictionary api
		resp, err := http.Get(fmt.Sprintf("https://api.dictionaryapi.dev/api/v2/entries/en/%s", word))
		if err != nil {
			fmt.Printf("error occured during search | %s\n", err.Error())
			os.Exit(1)
		}

		respData := SearchResults{}
		if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
			fmt.Printf("unable to find definition\n")
			return
		}

		if len(respData) == 0 {
			return
		}

		result = respData.String()

		// record search
		addResult(word, result)
	} 

	fmt.Println(result)
}

func main() {
	defer db.Close()

	if len(os.Args) <= 1 {
		fmt.Println("use command 'help' for a list of available commands")
		os.Exit(0)
	}

	command := strings.ToLower(os.Args[1])

	switch command {
	case HELP:
		help()
	case LIST:
		list()
	case SEARCH:
		search()
	default:
		fmt.Println("invalid command | use command 'help' for a list of available commands")
		os.Exit(0)
	}
}
