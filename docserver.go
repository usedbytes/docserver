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
	"github.com/codegangsta/cli"
	"github.com/shurcooL/github_flavored_markdown"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"text/template"
)

var pageTemplate *template.Template
var root string

const maxLinkLevels = 5

var indexes = []string{
	"index.md",
	"README.md",
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

	title, _ := filepath.Rel(root, file)
	page := &Page{
		Title:  title,
		Markup: string(github_flavored_markdown.Markdown(md)[:]),
	}
	err = pageTemplate.Execute(w, page)
	if err != nil {
		log.Printf("*-> Error: %s\n", err)
	}
	return
}

func handleFile(w http.ResponseWriter, r *http.Request, filename string) {
	ext := filepath.Ext(filename)
	if ext == ".md" {
		serveMarkdown(w, r, filename)
	} else {
		log.Printf("`-> Serving file: %s\n", filename)
		mimeType := mime.TypeByExtension(ext)
		if mimeType != "" {
			w.Header().Set("Content-Type", mimeType)
		}
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

func isSymLink(fi os.FileInfo) bool {
	return fi.Mode()&os.ModeSymlink != 0
}

func rootPath(path string, root string) string {
	return filepath.Clean(filepath.Join(root, path))
}

func resolvePath(path string) (newpath string, err error) {
	log.Printf("|-> Resolving: %s", path)
	fi, err := os.Lstat(path)
	if err != nil {
		return path, err
	}

	for level := 0; isSymLink(fi) && level < maxLinkLevels; level++ {
		target, err := os.Readlink(path)
		if err != nil {
			return path, err
		}
		if filepath.IsAbs(target) {
			path = target
			log.Printf("|-> Link to: %s", path)
		} else {
			path = filepath.Join(filepath.Dir(path), target)
			log.Printf("|-> Link to: %s (%s)", path, target)
		}

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

func resolveRequest(r *http.Request) (string, error) {
	p := filepath.Clean(filepath.Join(root, r.URL.Path))

	p, err := resolvePath(p)
	if err != nil {
		return "", err
	}

	// Resolve an index page if needed
	fi, err := os.Stat(p)
	if err != nil {
		return "", err
	} else if fi.IsDir() {
		// Search for indexes
		for _, i := range indexes {
			var index string
			log.Printf("|-> Find index: %s\n", filepath.Join(p, i))
			index, err = resolvePath(filepath.Join(p, i))
			if err == nil {
				p = index
				break
			}
		}
		// No index found
		if err != nil {
			return "", errors.New(fmt.Sprintf("Failed to get index for '%s'", p))
		}

		// Check the index isn't another directory
		fi, err = os.Stat(p)
		if err != nil {
			return "", err
		} else if fi.IsDir() {
			return "", errors.New(fmt.Sprintf("Found directory looking for '%s'",
				p))
		}
	}

	// Not allowed to traverse above root
	relp, err := filepath.Rel(root, p)
	if err != nil {
		log.Printf("|-> Couldn't get relative path: %s", p)
		return "", &RequestError{r.URL.Path, "No relative path",
			http.StatusNotFound}
	} else if len(relp) > 1 && relp[:2] == ".." {
		log.Printf("|-> Traverse above root: %s", relp)
		return "", os.ErrPermission
	}

	// Finally, check for access to the URL
	f, err := os.Open(p)
	if err == nil {
		defer f.Close()
	}

	return p, err
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s\n", dumpRequest(r))

	file, err := resolveRequest(r)
	if err != nil {
		handleError(w, r, err)
	} else {
		log.Printf("|-> Resolved: %s\n", file)
		handleFile(w, r, file)
	}
}

func runServer(c *cli.Context) {
	var err error

	templateFile := c.GlobalString("template")
	if templateFile == "" {
		pageTemplate, err = template.New("page").Parse(defaultPage)
	} else {
		pageTemplate, err = template.ParseFiles(templateFile)
	}
	if err != nil {
		log.Printf("Error parsing template: %s\n", err)
		return
	}

	root = c.GlobalString("root")
	log.Printf("Document root: %s\n", root)

	if c.GlobalBool("chroot") {
		log.Printf("`-> chroot() into document root\n")
		err = syscall.Chroot(root)
		if err != nil {
			log.Fatalf("chroot() failed: %s\n", err)
		}
		root = "/"
	}

	addr := c.GlobalString("addr")
	log.Printf("Serving on '%s'\n", addr)

	http.HandleFunc("/", handleRequest)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func main() {
	app := cli.NewApp()
	app.Name = "docserver"
	app.Usage = "Simple webserver for serving markdown files"
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "template",
			Value: "",
			Usage: "Template file for rendering Markdown pages - see text/template.\n" +
				"\tIf not provided, then a default template is used which defines a basic HTML page\n" +
				"\tAvailable variables:\n" +
				"\t\t.Title:  Page title\n" +
				"\t\t.Markup: HTML page content",
		},
		cli.StringFlag{
			Name:  "root",
			Value: ".",
			Usage: "Root directory to serve files from",
		},
		cli.StringFlag{
			Name:  "addr",
			Value: ":8000",
			Usage: "addr:port to listen on",
		},
		cli.BoolFlag{
			Name: "chroot",
			Usage: "If set, the server will chroot() to its document root " +
				   "upon starting",
		},
	}
	app.Action = runServer

	app.Run(os.Args)
}
