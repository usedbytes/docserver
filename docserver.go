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
	"net/http"
	"os"
	"path/filepath"
)

type RequestError struct {
	Url  string
	Msg  string
	Code int
}

func (e *RequestError) Error() string {
	return fmt.Sprintf("%s: %s", e.Url, e.Msg)
}

func handleError(w http.ResponseWriter, r *http.Request, err error) {
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

func serveMarkdown(w http.ResponseWriter, r *http.Request, file string) {
	md, err := ioutil.ReadFile(file)
	if err != nil {
		handleError(w, r, &RequestError{r.URL.Path, "Couldn't read file",
			http.StatusNotFound})
		return
	}

	mu := github_flavored_markdown.Markdown(md)
	l, err := w.Write(mu)
	if l != len(mu) || err != nil {
		fmt.Printf("Error writing file.\n")
	}
	return
}

func handleFile(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Path
	if filepath.Ext(filename) == ".md" {
		serveMarkdown(w, r, filename)
	} else {
		dat, err := ioutil.ReadFile(filename)
		if err != nil {
			handleError(w, r, &RequestError{r.URL.Path, "Couldn't read file",
				http.StatusNotFound})
			return
		}

		l, err := w.Write(dat)
		if l != len(dat) || err != nil {
			fmt.Printf("Error writing file.\n")
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
	request_path := r.URL.Path
	fmt.Printf("Request: %s\n", request_path)

	err := resolveRequest(r)
	if err != nil {
		handleError(w, r, err)
	} else {
		handleFile(w, r)
	}
}

func main() {
	http.HandleFunc("/", handleRequest)
	http.ListenAndServe(":8080", nil)
}
