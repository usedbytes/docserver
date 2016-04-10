# docserver

`docserver` is a simple markdown webserver. It will "markup" markdown documents
using http://github.com/shurcooL/github_flavored_markdown, and serve up the
HTML (including any non-markdown resources).

The page template for the HTML documents can be specified on the command line.
For instance the following template, saved as `template.html` will render all
markdown pages with a red background when `docserver --template template.html`
is executed:
```
<html>
	<head>
		<title>{{ .Title }}</title>
		<meta charset="utf-8">
	</head>
	<body style="background-color: red;">
		<article>
		{{ .Markup }}
		</article>
	</body>
</html>
```

## Features

* (hopefully) robust handling of symbolic links - requests cannot traverse
  outside of the document root
* Regular-expression-based request filtering
* Support for chroot()-ing into the document root (not actually sure if this
  is a good idea?)
* Files called `README.md` or `index.md` will be used for directory indexes
* Support for systemd socket activation

## Examples

Serve files out of the current directory on port 8000 (then try going to
`127.0.0.1:8000` to view this README):
```
$ docserver
```

Serve files out of `/srv/foo`, denying any requests which resolve to files with
`bar` or `baz` in their paths/names:
```
$ docserver --filter bar --filter baz --root /srv/foo
```

Serve files from `/srv/foo` on port 9000, and chroot() into `/srv/foo`:
```
# docserver --addr ':9000' --root /srv/foo --chroot
```

Run a socket activated server (systemd .socket and .service files):
```
# ======================
# File: docserver.socket
# ======================
[Unit]
Description=Docserver socket

[Socket]
ListenStream=80

[Install]
WantedBy=sockets.target
```
```
# =======================
# File: docserver.service
# =======================
[Unit]
Description=Docserver

[Service]
ExecStart=/path/to/docserver --root=/path/to/srv
```
