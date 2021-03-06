package main

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"text/template"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"github.com/google/go-github/github"
	"github.com/microcosm-cc/bluemonday"
)

// markdown to html by github Web API
func githubMdParse(input []byte) []byte {
	client := github.NewClient(nil)
	opt := &github.MarkdownOptions{Mode: "gfm", Context: "google/go-github"}
	maybeUnsafeHTML, _, err := client.Markdown(context.Background(), string(input), opt)
	if err != nil {
		log.Fatal(err)
	}

	output := bluemonday.UGCPolicy().SanitizeBytes([]byte(maybeUnsafeHTML))

	return output
}

// markdown to html by gomarkdown format
func goMdParse(input []byte) []byte {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs

	p := parser.NewWithExtensions(extensions)

	html := markdown.ToHTML(input, p, nil)
	log.Print(string(html))
	return html
}

func getBinDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.Dir(exe), nil
}

func getTitle(w http.ResponseWriter, r *http.Request) (string, error) {
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return "", errors.New("Invalid Page Title")
	}
	return m[2], nil // The title is the second subexpression
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func frontHandler(w http.ResponseWriter, r *http.Request, title string) {
	viewHandler(w, r, "frontPage")
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func getFileNameWithoutExt(path string) string {
	return filepath.Base(path[:len(path)-len(filepath.Ext(path))])
}

func getLatestPage() []string {
	files, err := ioutil.ReadDir(usrDir)
	if err != nil {
		panic(err)
	}

	sort.Sort(ByName{files})
	var paths []string
	for _, file := range files {
		paths = append(paths, getFileNameWithoutExt(file.Name()))
	}

	return paths
}

func init() {
	binDir, err := getBinDir()
	if err != nil {
		os.Exit(1)
	}
	usrDir = filepath.Join(binDir, `pages`)
	dataDir := filepath.Join(binDir, `data`)
	funcMaps := template.FuncMap{
		"getLatestPage": getLatestPage,
	}

	// read template file
	//for _, tmpl := range []string{"edit", "view", "frontPage"} {
	for _, tmpl := range []string{"edit", "view"} {
		file := tmpl + ".tmpl"
		t := template.Must(template.New(file).Funcs(funcMaps).ParseFiles(filepath.Join(dataDir, `tmpl`, file), filepath.Join(dataDir, `tmpl`, `content.tmpl`)))
		templates[tmpl] = t
	}
}

var templates = make(map[string]*template.Template)
var usrDir, dataDir string

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	//err := templates[tmpl].Execute(w, p)
	err := templates[tmpl].ExecuteTemplate(w, tmpl, p)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fn(w, r, "frontPage")
		} else {

			m := validPath.FindStringSubmatch(r.URL.Path)
			if m == nil {
				http.NotFound(w, r)
				return
			}
			fn(w, r, m[2])
		}
	}
}
