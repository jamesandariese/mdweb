package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	text_template "text/template"
	"time"
)

var funcMap = template.FuncMap{
	"markdown": renderMarkdownHelper,
}

var htmlTemplateEnv = &atomic.Value{}

var errorTemplateEnv = &atomic.Value{}
var mdTemplateEnv = &atomic.Value{}

func loadHtmlTemplates() {
	if tmpl, err := template.New("main.template").Funcs(funcMap).ParseGlob("templates/*.template"); err != nil {
		if htmlTemplateEnv.Load() == nil {
			log.Fatal("Couldn't load html templates:", err)
		}
		log.Print("Error reloading html templates:", err)
	} else {
		htmlTemplateEnv.Store(tmpl)
	}
}
func getHtmlTemplateEnv() *template.Template {
	return htmlTemplateEnv.Load().(*template.Template)
}
func getErrorTemplateEnv() *text_template.Template {
	return errorTemplateEnv.Load().(*text_template.Template)
}
func getMdTemplateEnv() *text_template.Template {
	return mdTemplateEnv.Load().(*text_template.Template)
}
func loadErrorTemplates() {
	if tmpl, err := text_template.New("main.md").ParseGlob("errors/*.md"); err != nil {
		if errorTemplateEnv.Load() == nil {
			log.Fatal("Couldn't load html templates:", err)
		}
		log.Print("Error reloading html templates:", err)
	} else {
		errorTemplateEnv.Store(tmpl)
	}
}
func loadMdTemplates() {
	if tmpl, err := text_template.New("main.md").ParseGlob("site/*.md"); err != nil {
		if mdTemplateEnv.Load() == nil {
			log.Fatal("Couldn't load html templates:", err)
		}
		log.Print("Error reloading html templates:", err)
	} else {
		mdTemplateEnv.Store(tmpl)
	}
}

func loadTemplates() {
	// html templates can contain markdown.  reload it first or it'll be delayed by 10s
	loadMdTemplates()
	loadHtmlTemplates()
	loadErrorTemplates()
}

func reload() {
	for {
		time.Sleep(10 * time.Second)
		loadTemplates()
	}
}

func init() {
	loadTemplates()
	go reload()
}

type Error struct {
	Code int
}

var ErrNotFound = errors.New("Not Found")

func renderMarkdown(path string, extra interface{}) ([]byte, error) {

	mdbuf := &bytes.Buffer{}

	myErrorTemplateEnv := getErrorTemplateEnv()
	myMdTemplateEnv := getMdTemplateEnv()

	switch extra := extra.(type) {
	case *Error:
		if myErrorTemplateEnv.Lookup(path) == nil {
			return nil, ErrNotFound
		}
		if err := myErrorTemplateEnv.ExecuteTemplate(mdbuf, path, extra); err != nil {
			return nil, err
		}
	default:
		if myMdTemplateEnv.Lookup(path) == nil {
			return nil, ErrNotFound
		}
		if err := myMdTemplateEnv.ExecuteTemplate(mdbuf, path, extra); err != nil {
			return nil, err
		}
	}

	unsafe := blackfriday.MarkdownCommon(mdbuf.Bytes())
	return unsafe, nil
}

func render(path string, extra interface{}) ([]byte, error) {
	unsafe_bytes, err := renderMarkdown(path, extra)
	if err != nil {
		return nil, err
	}
	html := template.HTML(bluemonday.UGCPolicy().SanitizeBytes(unsafe_bytes))

	var typ string
	switch extra.(type) {
	case *Error:
		typ = "error"
	default:
		typ = "main"
	}

	buf := &bytes.Buffer{}
	if err := getHtmlTemplateEnv().ExecuteTemplate(buf, typ+".template", html); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderMarkdownHelper(path string, extra ...interface{}) (template.HTML, error) {
	var first_extra interface{}
	if len(extra) > 0 {
		first_extra = extra[0]
	}
	unsafe_bytes, err := renderMarkdown(path, first_extra)
	//return //template.HTML(string(x)), y
	return template.HTML(bluemonday.UGCPolicy().SanitizeBytes(unsafe_bytes)), err
}

func handleError(w http.ResponseWriter, r *http.Request, code int, message string) {
	w.WriteHeader(code)
	errorPage := fmt.Sprintf("%d.md", code)
	if byts, err := render(errorPage, &Error{code}); err != nil {
		//w.Write([]byte("???"))
		fmt.Fprintf(w, "%d: %v\n", code, err)
	} else {
		w.Write(byts)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.RequestURI)
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/index", 302)
		return
	}
	if strings.Contains(r.URL.Path, "/..") {
		log.Println("Blocked bad URL:", r.URL.Path)
		handleError(w, r, 404, r.URL.Path)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/static/") {
		in, err := os.Open(r.URL.Path[1:])
		if err != nil {
			w.WriteHeader(404)
			return
		}
		defer in.Close()
		http.ServeContent(w, r, r.URL.Path, time.Time{}, in)
		return
	}
	// Here lies a race condition.  The solution is either to pass the
	// current mdTemplateEnv down or lock it for the duration of the
	// request.
	// The hack is to accept that a 500 might rarely happen if a template
	// is *removed* during a reload.
	// This also will never happen when operating without parallelism
	// (so use many instances and load balance when load becomes high
	// enough to require multiple cores).
	if getMdTemplateEnv().Lookup(r.URL.Path[1:]+".md") == nil {
		handleError(w, r, 404, r.URL.Path)
		return
	}
	if byts, err := render(r.URL.Path[1:]+".md", nil); err != nil {
		log.Printf("Error serving: %v", err)
		handleError(w, r, 500, err.Error())
		return
	} else {
		w.Write(byts)
	}
}

func main() {
	http.HandleFunc("/", handler)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
