//  Copyright Brian Starkey <stark3y@gmail.com> 2016
//
//  Permission is hereby granted, free of charge, to any person obtaining a
//  copy of this software and associated documentation files (the "Software"),
//  to deal in the Software without restriction, including without limitation
//  the rights to use, copy, modify, merge, publish, distribute, sublicense,
//  and/or sell copies of the Software, and to permit persons to whom the
//  Software is furnished to do so, subject to the following conditions:
//
//  The above copyright notice and this permission notice shall be included in
//  all copies or substantial portions of the Software.
//
//  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
//  OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
//  FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
//  DEALINGS IN THE SOFTWARE.
//
// docserver provides a basic Markdown document webserver
package main

import (
	"errors"
	"fmt"
	"github.com/shurcooL/github_flavored_markdown"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
)

type RequestError struct {
	Url  string
	Msg  string
	Code int
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("%s: %s", e.Url, e.Msg)
}

func dumpRequest(r *http.Request) string {
	return fmt.Sprintf("From: %s -> %s %s%s",
		r.RemoteAddr, r.Method, r.Host, r.RequestURI)
}

func handleError(w http.ResponseWriter, r *http.Request, err error) {

	log.Printf("`-> Error: %s '%s'\n", r.URL.Path, err.Error())

	switch e := err.(type) {
	case *os.PathError, *os.LinkError:
		if os.IsNotExist(err) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if os.IsPermission(err) {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case *RequestError:
		http.Error(w, err.Error(), e.Code)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

const defaultPage string = `
<html>
	<head>
		<title>{{ .Title }}</title>
		<meta charset="utf-8">
	</head>
	<body>
		<article>
		{{ .Markup }}
		</article>
	</body>
</html>
`

var pageTemplate *template.Template

type Page struct {
	Title  string
	Markup string
}

func serveMarkdown(w http.ResponseWriter, r *http.Request, file string) {
	log.Printf("`-> Serving markdown: %s\n", file)
	md, err := ioutil.ReadFile(file)
	if err != nil {
		handleError(w, r, &RequestError{r.URL.Path, "Couldn't read file",
			http.StatusNotFound})
		return
	}

	page := &Page{
		Title:  file,
		Markup: string(github_flavored_markdown.Markdown(md)[:]),
	}
	err = pageTemplate.Execute(w, page)
	if err != nil {
		log.Printf("*-> Error: %s\n", err)
	}
	return
}

func handleFile(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Path
	if filepath.Ext(filename) == ".md" {
		serveMarkdown(w, r, filename)
	} else {
		log.Printf("`-> Serving file: %s\n", filename)
		dat, err := ioutil.ReadFile(filename)
		if err != nil {
			handleError(w, r, &RequestError{r.URL.Path, "Couldn't read file",
				http.StatusNotFound})
			return
		}

		l, err := w.Write(dat)
		if l != len(dat) || err != nil {
			log.Printf("*-> Error: wrote %d of %d bytes\n", l, len(dat))
		}
		return
	}
}

const maxLinkLevels = 5

func isSymLink(fi os.FileInfo) bool {
	return fi.Mode()&os.ModeSymlink != 0
}

func rootPath(path string, root string) string {
	return filepath.Clean(filepath.Join(root, path))
}

func resolvePath(path string, root string) (newpath string, err error) {
	path = rootPath(path, root)
	fi, err := os.Lstat(path)
	if err != nil {
		return path, err
	}

	for level := 0; isSymLink(fi) && level < maxLinkLevels; level++ {
		path, err = os.Readlink(path)
		if err != nil {
			return path, err
		}
		log.Printf("|-> Link to: %s", path)
		path = rootPath(path, root)

		fi, err = os.Lstat(path)
		if err != nil {
			return path, err
		}

		if !isSymLink(fi) {
			break
		}
	}
	if isSymLink(fi) {
		return path, errors.New("Too many levels of indirection")
	}

	return path, nil
}

var indexes = []string{
	"index.md",
	"README.md",
}

const Root string = "."

func resolveRequest(r *http.Request) error {
	p := filepath.Clean(filepath.Join(Root, r.URL.Path))

	p, err := resolvePath(p, Root)
	if err != nil {
		return err
	}

	// Resolve an index page if needed
	fi, err := os.Stat(p)
	if err != nil {
		return err
	} else if fi.IsDir() {
		// Search for indexes
		for _, i := range indexes {
			var index string
			log.Printf("|-> Find index: %s\n", filepath.Join(p, i))
			index, err = resolvePath(filepath.Join(p, i), Root)
			if err == nil {
				p = index
				break
			}
		}
		// No index found
		if err != nil {
			return errors.New(fmt.Sprintf("Failed to get index for '%s'", p))
		}

		// Check the index isn't another directory
		fi, err = os.Stat(p)
		if err != nil {
			return err
		} else if fi.IsDir() {
			return errors.New(fmt.Sprintf("Found directory looking for '%s'",
				p))
		}
	}

	// Not allowed to traverse above Root
	p, err = filepath.Rel(Root, p)
	if len(p) > 1 && p[:2] == ".." {
		log.Printf("|-> Traverse above root: %s", p)
		return os.ErrPermission
	}

	// Finally, check for access to the URL
	f, err := os.Open(p)
	if err == nil {
		r.URL.Path = p
		defer f.Close()
	}

	return err
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s\n", dumpRequest(r))

	err := resolveRequest(r)
	if err != nil {
		handleError(w, r, err)
	} else {
		log.Printf("|-> Resolved: %s\n", r.URL.Path)
		handleFile(w, r)
	}
}

func main() {
	var err error
	pageTemplate, err = template.New("page").Parse(defaultPage)
	if err != nil {
		log.Printf("Error parsing template: %s\n", err)
	}

	port := ":8080"
	log.Printf("Serving on port '%s'\n", port)
	http.HandleFunc("/", handleRequest)
	http.ListenAndServe(port, nil)
}
