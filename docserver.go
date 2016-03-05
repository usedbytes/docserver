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
	"fmt"
	"github.com/shurcooL/github_flavored_markdown"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

func serveError(err error, w http.ResponseWriter) {
	fmt.Fprintf(w, "Error: %s", err)
}

func serveMarkdown(file string, w http.ResponseWriter) {
		md, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Fprintf(w, "Couldn't read %s", file)
			return
		}

		mu := github_flavored_markdown.Markdown(md)
		l, err := w.Write(mu)
		if l != len(mu) || err != nil {
			fmt.Printf("Error writing file.\n")
		}
		return
}

func serveFile(file string, w http.ResponseWriter) {
	if filepath.Ext(file) == ".md" {
		serveMarkdown(file, w)
	} else {
		dat, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Fprintf(w, "Couldn't read %s", file)
			return
		}

		l, err := w.Write(dat)
		if l != len(dat) || err != nil {
			fmt.Printf("Error writing file.\n")
		}
		return
	}
}

func validateFile(filename string) (newname string, err error) {
	fmt.Printf("Validate '%s'\n", filename)
	fi, err := os.Lstat(filename)
	if err != nil {
		return filename, err
	}

	if (fi.Mode() & os.ModeSymlink) != 0 {
		for level := 0; level < 5; level++ {
			filename, err = os.Readlink(filename)
			if err != nil {
				return filename, err
			}

			fi, err := os.Lstat(filename)
			if err != nil {
				return filename, err
			}

			if (fi.Mode() & os.ModeSymlink) == 0 {
				break;
			}
		}
	}

	filename = filepath.Clean(filepath.Join(".", filename))
	if len(filename) > 1 && filename[:2] == ".." {
		return filename, os.ErrPermission
	}

	f, err := os.Open(filename)
	if err == nil {
		defer f.Close()
	}
	return filename, err
}

var indexes = []string{
	"index.md",
	"README.md",
};

func handleRequest(w http.ResponseWriter, r *http.Request) {
	p := filepath.Join(".", r.URL.Path)
	p = filepath.Clean(p)

	p, err := validateFile(p)
	if err != nil {
		serveError(err, w)
		return
	}

	fi, err := os.Stat(p)
	if err != nil {
		serveError(err, w)
		return;
	} else if fi.IsDir() {
		// Search for indexes
		for _, i := range indexes {
			var index string
			index, err = validateFile(filepath.Join(p, i))
			if err == nil {
				p = index
				break;
			}
		}
		if err != nil {
			serveError(err, w)
			return
		}
	}

	serveFile(p, w)
}

func main() {
	http.HandleFunc("/", handleRequest)
	http.ListenAndServe(":8080", nil)
}
