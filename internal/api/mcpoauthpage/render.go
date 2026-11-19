// Package mcpoauthpage renders the page a browser lands on after an MCP OAuth redirect reaches the
// daemon's loopback callback. The page is standalone: its styles, fonts and brand images are
// embedded and inlined as data URIs, so it needs no other route and runs under a CSP that only
// admits its own hashed style and script.
package mcpoauthpage

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"strings"
)

//go:embed page.html.tmpl page.css page.js assets/*
var files embed.FS

// The stylesheet (self-hosted faces, generated theme tokens, layout), script, images and CSP are
// fixed for the process, so the style and script hashes are computed once.
var (
	pageTemplate = template.Must(template.ParseFS(files, "page.html.tmpl"))
	pageCSS      = buildCSS()
	pageScript   = mustRead("page.js")
	symbolURI    = dataURI("image/png", "assets/symbol.png")
	convergeURI  = dataURI("image/jpeg", "assets/converge.jpg")
	horizonURI   = dataURI("image/jpeg", "assets/horizon.jpg")
	policy       = contentSecurityPolicy()
)

var fontFaces = []struct {
	family string
	file   string
}{
	{family: "Figtree Variable", file: "assets/figtree.woff2"},
	{family: "Instrument Sans Variable", file: "assets/instrument-sans.woff2"},
	{family: "Geist Mono Variable", file: "assets/geist-mono.woff2"},
}

// ContentSecurityPolicy is the policy every rendered page must be served with.
func ContentSecurityPolicy() string {
	return policy
}

// Render writes the callback page for one outcome.
func Render(w io.Writer, page Page) error {
	var buf bytes.Buffer
	if err := pageTemplate.Execute(&buf, newView(page)); err != nil {
		return fmt.Errorf("mcpoauthpage: render: %w", err)
	}
	if _, err := buf.WriteTo(w); err != nil {
		return fmt.Errorf("mcpoauthpage: write: %w", err)
	}
	return nil
}

func buildCSS() string {
	var css strings.Builder
	for _, face := range fontFaces {
		fmt.Fprintf(
			&css,
			"@font-face{font-family:%q;font-style:normal;font-weight:100 900;font-display:swap;src:url(%s) format(\"woff2\")}\n",
			face.family,
			dataURI("font/woff2", face.file),
		)
	}
	css.WriteString(mustRead("assets/tokens.gen.css"))
	css.WriteString(mustRead("page.css"))
	return css.String()
}

func contentSecurityPolicy() string {
	return strings.Join([]string{
		"default-src 'none'",
		"style-src '" + sourceHash(pageCSS) + "'",
		"script-src '" + sourceHash(pageScript) + "'",
		"img-src data:",
		"font-src data:",
		"base-uri 'none'",
		"form-action 'none'",
		"frame-ancestors 'none'",
	}, "; ")
}

func sourceHash(source string) string {
	sum := sha256.Sum256([]byte(source))
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}

func dataURI(mediaType, name string) string {
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString([]byte(mustRead(name)))
}

func mustRead(name string) string {
	data, err := files.ReadFile(name)
	if err != nil {
		panic("invariant: mcpoauthpage embeds " + name + ": " + err.Error())
	}
	return string(data)
}
