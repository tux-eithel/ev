package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/pkg/browser"
	"github.com/tux-eithel/ev"
	"github.com/tux-eithel/ev/ui"
)

func usageAndExit() {
	fmt.Println("usage:\n\tev <funcname>:<file> or <start_line>,<end_line>:<file>")
	os.Exit(0)
}

func main() {
	var err error
	log.SetFlags(0)
	log.SetPrefix("ev: ")
	if len(os.Args) <= 1 {
		usageAndExit()
	}
	parts := strings.Split(os.Args[1], ":")
	if len(parts) != 2 {
		usageAndExit()
	}
	funcName, fileName := parts[0], parts[1]

	parsedLog, err := ev.Log(funcName, fileName)
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/dist/", http.StripPrefix("/dist/", http.FileServer(ui.FS)))

	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if err := indexTemplate.Execute(w, parsedLog); err != nil {
			log.Fatal(err)
		}
	})

	go func() {
		if err := http.ListenAndServe(":8888", mux); err != nil {
			log.Fatal(err)
		}
	}()
	browser.OpenURL("http://localhost:8888")
	select {}
}
