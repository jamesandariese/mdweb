package main

import (
	"fmt"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

var templateEnv = template.Must(template.New("foo").ParseGlob("templates/*.template"))

func handler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/static/") {
		in, err := os.Open(r.URL.Path[1:])
		if err != nil {
			w.WriteHeader(404)
			return
		}
		if n, err := io.Copy(w, in); err != nil {
			log.Printf("Wrote %d bytes before error: %v", n, err)
		}
		return
	}

	if byts, err := ioutil.ReadFile("site/" + r.URL.Path[1:]); err != nil {
		w.WriteHeader(404)
	} else {
		unsafe := blackfriday.MarkdownCommon(byts)
		html := template.HTML(bluemonday.UGCPolicy().SanitizeBytes(unsafe))
		err = templateEnv.ExecuteTemplate(w, "main.template", html)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "%v\n", err)
		}
	}
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}
