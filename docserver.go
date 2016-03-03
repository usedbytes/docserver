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
	"path/filepath"
)

func handleRequest(w http.ResponseWriter, r *http.Request) {
	p := filepath.Clean(r.URL.Path)

	if filepath.IsAbs(p) {
		p = p[1:]
	} else if p[:2] == ".." {
		fmt.Fprintf(w, "Not allowed traverse upwards! %s", r.URL.Path)
		return
	}

	if filepath.Ext(p) == ".md" {
		md, err := ioutil.ReadFile(p)
		if err != nil {
			fmt.Fprintf(w, "Couldn't read %s", p)
			return
		}

		mu := github_flavored_markdown.Markdown(md)
		l, err := w.Write(mu)
		if l != len(mu) || err != nil {
			fmt.Printf("Error writing file.\n")
		}
		return
	}

	fmt.Fprintf(w, "Don't know what to do for %s", p)
}

func main() {
	http.HandleFunc("/", handleRequest)
	http.ListenAndServe(":8080", nil)
}
