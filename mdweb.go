package main

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	//"github.com/microcosm-cc/bluemonday"
	"flag"
	"github.com/russross/blackfriday"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	texttemplate "text/template"
	"time"
)

type Page struct {
	R        *http.Request
	HTML     template.HTML
	Template string
}

var listen = flag.String("listen", ":4080", "listen address")
var site = flag.String("site", "./site", "site directory")
var staticroot = flag.Bool("static-root", true, "alias static/* to /* (slightly slower)")

var funcMap = template.FuncMap{
	"markdown": renderMarkdownHelper,
}

var textFuncMap = texttemplate.FuncMap{
	"markdown": renderMarkdownHelper,
}

var htmlTemplateEnv = &atomic.Value{}

var errorTemplateEnv = &atomic.Value{}
var mdTemplateEnv = &atomic.Value{}

func loadHtmlTemplates() {
	if tmpl, err := template.New("main.template").Funcs(funcMap).ParseGlob(filepath.Join(*site, "templates/*.template")); err != nil {
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
func getErrorTemplateEnv() *texttemplate.Template {
	return errorTemplateEnv.Load().(*texttemplate.Template)
}
func getMdTemplateEnv() *texttemplate.Template {
	return mdTemplateEnv.Load().(*texttemplate.Template)
}
func loadErrorTemplates() {
	if tmpl, err := texttemplate.New("main.md").ParseGlob(filepath.Join(*site, "errors/*.md")); err != nil {
		if errorTemplateEnv.Load() == nil {
			log.Fatal("Couldn't load error templates:", err)
		}
		log.Print("Error reloading error templates:", err)
	} else {
		errorTemplateEnv.Store(tmpl)
	}
}
func loadMdTemplates() {
	tmpl := texttemplate.New("nil").Funcs(textFuncMap)
	success := false
	if files, err := filepath.Glob(filepath.Join(*site, "*.pmd")); err == nil && len(files) > 0 {
		if _, err := tmpl.ParseFiles(files...); err != nil {
			log.Print("Couldn't load pmd templates:", err)
		} else {
			success = true
		}
	}
	if files, err := filepath.Glob(filepath.Join(*site, "*.md")); err == nil && len(files) > 0 {
		if _, err := tmpl.ParseFiles(files...); err != nil {
			log.Print("Couldn't load md templates:", err)
		} else {
			success = true
		}
	}
	if !success {
		log.Print("Couldn't load any md or pmd templates")
	} else {
		mdTemplateEnv.Store(tmpl)
	}
}

var templateOnceLoader sync.Once

func loadTemplates() {
	// html templates can contain markdown.  reload it first or it'll be delayed by 10s
	loadMdTemplates()
	loadHtmlTemplates()
	loadErrorTemplates()
	templateOnceLoader.Do(func() { go reload() })
}

func reload() {
	for {
		time.Sleep(10 * time.Second)
		loadTemplates()
	}
}

type Error struct {
	Code int
	Path string
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

func render(path string, extra interface{}, r *http.Request) ([]byte, error) {
	unsafe_bytes, err := renderMarkdown(path, extra)
	if err != nil {
		return nil, err
	}
	//html := template.HTML(bluemonday.UGCPolicy().SanitizeBytes(unsafe_bytes))
	html := template.HTML(unsafe_bytes)

	var typ string
	switch extra.(type) {
	case *Error:
		typ = "error"
	default:
		typ = "main"
	}

	buf := &bytes.Buffer{}
	if err := getHtmlTemplateEnv().ExecuteTemplate(buf, typ+".template", &Page{r, html, typ}); err != nil {
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
	//return template.HTML(bluemonday.UGCPolicy().SanitizeBytes(unsafe_bytes)), err
	return template.HTML(unsafe_bytes), err
}

func handleError(w http.ResponseWriter, r *http.Request, code int, message string) {
	w.WriteHeader(code)
	errorPage := fmt.Sprintf("%d.md", code)
	if byts, err := render(errorPage, &Error{code, r.URL.Path}, r); err != nil {
		//w.Write([]byte("???"))
		fmt.Fprintf(w, "%d: %v\n", code, err)
	} else {
		w.Write(byts)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.RequestURI)

	r.ParseForm()

	if r.URL.Path == "/" {
		r.URL.Path = "/index"
	}

	if *staticroot {
		if fi, err := os.Stat(filepath.Join(*site, "static", r.URL.Path[1:])); err == nil {
			if fi.Mode().IsRegular() {
				log.Printf("Aliasing %s to /static%s", r.URL.Path, r.URL.Path)
				r.URL.Path = "/static" + r.URL.Path
			}
		}
	} else {
		// transform the path a bit, if needed
		switch r.URL.Path {
		case "/robots.txt",
			"/favicon.ico":
			r.URL.Path = "/static" + r.URL.Path
		}
	}

	if strings.Contains(r.URL.Path, "/..") {
		log.Println("Blocked bad URL:", r.URL.Path)
		handleError(w, r, 404, r.URL.Path)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/static/") {
		in, err := os.Open(filepath.Join(*site, r.URL.Path[1:]))
		if err != nil {
			handleError(w, r, 404, r.URL.Path)
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
	if byts, err := render(r.URL.Path[1:]+".md", nil, r); err != nil {
		log.Printf("Error serving: %v", err)
		handleError(w, r, 500, err.Error())
		return
	} else {
		w.Write(byts)
	}
}

func main() {
	flag.Parse()
	loadTemplates()
	http.HandleFunc("/", handler)
	if err := http.ListenAndServe(*listen, nil); err != nil {
		log.Fatal(err)
	}
}
