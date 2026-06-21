// Copyright (C) 2024 glowinthedark <glwnd2030@gmail.com>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, write to the Free Software Foundation, Inc.,
// 51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.

// -------------------------------------------------------------------------

// MDict dictionary self-contained HTTP server.
//
// A single binary serves the search page, handles ?q / ?path / ?max queries,
// and serves .mdd resources extracted on demand into assetRoot.
//
// Configuration priority (highest → lowest):
//
//	CLI flag  >  environment variable  >  config.toml  >  built-in default
//
// Config file search order:
//
//	--config / CONFIG_PATH  →  <exe-dir>/config.toml  →  ~/.mdict/config.toml
//	→  /etc/mdict/config.toml  →  ./config.toml
//
// Param           CLI flag        Env var                TOML key               Default
// ─────────────────────────────────────────────────────────────────────────────────────
// dict dir        --dict-dir      DICT_DIR               DICT_DIR               ~/Dictionaries
// asset dir       --asset-dir     MDICT_TEMP_ASSETS_DIR  MDICT_TEMP_ASSETS_DIR  ~/.mdict/res
// default dict    --default-dict  DEFAULT_DICT           DEFAULT_DICT           (none, rel. to dict-dir)
// server IP       --ip            SERVER_IP              SERVER_IP              127.0.0.1
// server port     --port          SERVER_PORT            SERVER_PORT            8808
// speexdec path   --speexdec      SPEEXDEC               SPEEXDEC               /usr/bin/speexdec
// no browser      --no-browser    NO_BROWSER=1           NO_BROWSER=1           (open browser)
// config file     --config        CONFIG_PATH            —                      (auto-detect)
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/browser"
)

//go:embed web/mdict.html
var templateHTML string

//go:embed web/mark.min.js
var markJS []byte

const appVersion = "0.55"

const (
	maxItemsDefault        = 55
	configFileName         = "config.toml"
	dictNamePlaceholder    = "$$${{{DICT_NAME}}}"
	dictOptionsPlaceholder = "$$${{{DICT_OPTIONS}}}"
	dictBodyPlaceholder    = "$$${{{DICT_BODY}}}"
)

// Configs
var (
	dictDir      string // absolute path to .mdx/.mdd directory
	assetRoot    string // absolute path to extracted-asset cache
	defaultDict  string // absolute path to default dictionary (joined from dictDir + rel)
	speexDecPath string // absolute path to speexdec binary
	noBrowser    bool   // do not open browser on startup
)

// Reader cache: server is long-lived; each dictionary parsed once, reused across requests.
var (
	readerCacheMu sync.Mutex
	readerCache   = map[string]*Reader{}
)

func getReader(p string) (*Reader, error) {
	readerCacheMu.Lock()
	defer readerCacheMu.Unlock()
	if r, ok := readerCache[p]; ok {
		return r, nil
	}
	r, err := openReader(p)
	if err != nil {
		return nil, err
	}
	readerCache[p] = r
	return r, nil
}

// conf resolves a parameter in this priority chain:
// CLI flag → env var → config.toml key → built-in fallback.
// fval/fset come from flag.FlagSet: fset is true only when the user explicitly passed the flag.
func conf(fval string, fset bool, key, fallback string) string {
	if fset && fval != "" {
		return fval
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	return getConf(key, fallback)
}

// mustAbs expands ~ and resolves p to an absolute path.
// Returns p unchanged on empty input or resolution failure.
func mustAbs(p string) string {
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func main() {
	// ── CLI args parsing ───────────────────────────────────────────────────────────────
	fset := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fset.Usage = usageFn(fset)

	fConfig := fset.String("config", "", "path to config.toml (overrides auto-detect)")
	fDictDir := fset.String("dict-dir", "", "directory containing .mdx/.mdd files")
	fAssetDir := fset.String("asset-dir", "", "directory for extracted .mdd resources (cache)")
	fDefaultDict := fset.String("default-dict", "", "default dictionary, relative to dict-dir (e.g. en/Oxford.mdx)")
	fIP := fset.String("ip", "", "server listen IP")
	fPort := fset.String("port", "", "server listen port")
	fSpeexDec := fset.String("speexdec", "", "path to speexdec binary (Speex audio decoding)")
	fNoBrowser := fset.Bool("no-browser", false, "do not open browser on startup")
	fVersion := fset.Bool("version", false, "print version and exit")
	fset.BoolVar(fVersion, "v", false, "print version and exit (shorthand)")

	if err := fset.Parse(os.Args[1:]); err != nil {
		os.Exit(0)
	}

	if *fVersion {
		fmt.Printf("mdict-go-web v%s\n", appVersion)
		os.Exit(0)
	}

	set := map[string]bool{}
	fset.Visit(func(f *flag.Flag) { set[f.Name] = true })

	// ── Config file ──────────────────────────────────────────────────────────
	configPath := *fConfig
	if configPath == "" {
		configPath = getConfigPath()
	}
	LoadConfig(configPath)

	// ── Resolve parameters (CLI > env > toml > default) ─────────────────────
	// Path params run through mustAbs (handles ~ expansion + absolutization).
	dictDir = mustAbs(conf(*fDictDir, set["dict-dir"], "DICT_DIR", "~/Dictionaries"))
	assetRoot = mustAbs(conf(*fAssetDir, set["asset-dir"], "MDICT_TEMP_ASSETS_DIR", "~/.mdict/res"))
	speexDecPath = mustAbs(conf(*fSpeexDec, set["speexdec"], "SPEEXDEC", "/usr/local/bin/speexdec"))

	// defaultDict is relative to dictDir — join after both are resolved.
	if rel := conf(*fDefaultDict, set["default-dict"], "DEFAULT_DICT", ""); rel != "" {
		defaultDict = mustAbs(filepath.Join(dictDir, rel))
	}

	// server listen config
	ip := conf(*fIP, set["ip"], "SERVER_IP", "127.0.0.1")
	port := conf(*fPort, set["port"], "SERVER_PORT", "8808")

	// --no-browser
	envVal := getConf("NO_BROWSER", os.Getenv("NO_BROWSER"))
	envValFlag, _ := strconv.ParseBool(envVal)

	noBrowser = *fNoBrowser || envValFlag

	// ── Asset dir bootstrap ──────────────────────────────────────────────────
	if !dirExists(assetRoot) {
		fmt.Fprintf(os.Stderr, "creating asset dir %s\n", assetRoot)
		if err := os.MkdirAll(assetRoot, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "could not create asset dir %s: %v\n", assetRoot, err)
		}
	}

	// ── Startup summary ──────────────────────────────────────────────────────
	addr := ip + ":" + port
	fmt.Printf("version:   v%s\n", appVersion)
	fmt.Printf("config:    %s\n", configPath)
	fmt.Printf("dict-dir:  %s\n", dictDir)
	fmt.Printf("asset-dir: %s\n", assetRoot)
	fmt.Printf("speexdec:  %s\n", speexDecPath)

	// ── HTTP server ──────────────────────────────────────────────────────────
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not bind %s: %v\n", addr, err)
		os.Exit(1)
	}
	url := "http://" + addr + "/"
	fmt.Printf("listening: %s\n", url)

	if !noBrowser {
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintf(os.Stderr, "could not open browser: %v\n", err)
		}
	}

	if err := http.Serve(ln, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

// usage
func usageFn(_ *flag.FlagSet) func() {
	return func() {
		fmt.Fprintf(os.Stderr, `mdict-go-web v%s — MDict (.mdx/.mdd) HTTP dictionary server

USAGE`, appVersion)
		fmt.Fprint(os.Stderr, `
  mdict-go-web [flags]

FLAGS
  --config       <path>   Path to config.toml (overrides auto-detect)
                          env: CONFIG_PATH

  --dict-dir     <path>   Directory with .mdx/.mdd dictionary files
                          env: DICT_DIR              toml: DICT_DIR
                          default: ~/Dictionaries

  --asset-dir    <path>   Cache dir for extracted .mdd resources
                          env: MDICT_TEMP_ASSETS_DIR  toml: MDICT_TEMP_ASSETS_DIR
                          default: ~/.mdict/res

  --default-dict <rel>    Default dictionary, relative to dict-dir
                          e.g. "en/Oxford.mdx"
                          env: DEFAULT_DICT           toml: DEFAULT_DICT

  --ip           <addr>   Listen IP address
                          env: SERVER_IP              toml: SERVER_IP
                          default: 127.0.0.1

  --port         <port>   Listen port
                          env: SERVER_PORT            toml: SERVER_PORT
                          default: 8808

  --speexdec     <path>   Path to speexdec binary (Speex audio decoding)
                          env: SPEEXDEC               toml: SPEEXDEC
                          default: /usr/bin/speexdec

  --no-browser            Do not open a browser tab on startup
                          env/toml: NO_BROWSER=1

  -v, --version           Show version and exit

  -h, --help              Show this help and exit

CONFIG FILE SEARCH ORDER
  1. --config flag / CONFIG_PATH env var
  2. <executable-dir>/config.toml
  3. ~/.mdict/config.toml
  4. /etc/mdict/config.toml
  5. ./config.toml

PRIORITY (highest → lowest)
  CLI flag  >  environment variable  >  config.toml  >  built-in default

EXAMPLE config.toml
  DICT_DIR              = "/data/dicts"
  MDICT_TEMP_ASSETS_DIR = "/tmp/mdict-res"
  DEFAULT_DICT          = "en/Oxford.mdx"
  SERVER_IP             = "0.0.0.0"
  SERVER_PORT           = "9000"
  SPEEXDEC              = "/usr/bin/speexdec"
  NO_BROWSER            = "1"

EXAMPLES
  mdict-go-web
  mdict-go-web --dict-dir ~/Books/Dicts --port 9090 --no-browser
  mdict-go-web --config /etc/mdict/config.toml --default-dict "en/Oxford.mdx"
  SERVER_PORT=9000 mdict-go-web
`)
	}
}

// ── HTTP ─────────────────────────────────────────────────────────────
func handleRoot(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/":
		handlePage(w, r)
	case "/mark.min.js":
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write(markJS)
	default:
		serveAsset(w, r)
	}
}

// serveAsset serves extracted .mdd resources from assetRoot.
// Relative refs like "img/x.png" inside definitions resolve here.
func serveAsset(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if rel == "" || rel == "." || strings.HasPrefix(rel, "..") {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(assetRoot, filepath.FromSlash(rel)))
}

func handlePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if !dirExists(dictDir) {
		fmt.Fprintf(w, `<h3 style='color:red'><tt>DICT_DIR</tt> not found: %s</h3>
<p>Set the dictionary directory via one of:
<ol>
<li>CLI:     <tt>--dict-dir /path/to/dicts</tt></li>
<li>Env var: <tt>DICT_DIR=/path/to/dicts</tt></li>
<li>in <tt>config.toml<tt>:    <tt>DICT_DIR = "/path/to/dicts"</tt></li>
</ol>
<pre>
CONFIG FILE SEARCH ORDER
  1. --config flag / CONFIG_PATH env var
  2. <executable-dir>/config.toml
  3. %s/.mdict/config.toml
  4. /etc/mdict/config.toml
  5. ./config.toml
  </pre>
	`, dictDir, os.UserHomeDir())
		return
	}

    qv := r.URL.Query()
    q := qv.Get("q")
    maxN := maxItemsDefault
    if m := qv.Get("max"); m != "" {
        if n, err := strconv.Atoi(m); err == nil {
            maxN = n
        }
    }

    offset := 0
    if o := qv.Get("offset"); o != "" {
        if n, err := strconv.Atoi(o); err == nil && n >= 0 {
            offset = n
        }
    }

	// ?path= is relative to dictDir; falls back to configured defaultDict (already absolute).
	var absPathIn string
	if rel := qv.Get("path"); rel != "" {
		absPathIn = mustAbs(filepath.Join(dictDir, rel))
	} else {
		absPathIn = defaultDict
	}

	var reader *Reader
	if isFile(absPathIn) {
		if rd, err := getReader(absPathIn); err != nil {
			fmt.Fprintf(os.Stderr, "open dictionary %s: %v\n", absPathIn, err)
		} else {
			reader = rd
		}
	}

	// relPath: dictDir-relative path for the current dictionary, used by keyword links.
	relPath := absPathIn
	if reader != nil {
		if rp, err := filepath.Rel(dictDir, absPathIn); err == nil {
			relPath = filepath.ToSlash(rp)
		}
	}

	name := ""
	if reader != nil {
		name = reader.name
	}

	page := templateHTML
	page = strings.ReplaceAll(page, dictNamePlaceholder, html.EscapeString(name))
	page = strings.ReplaceAll(page, dictOptionsPlaceholder, buildOptions(absPathIn))
    page = strings.ReplaceAll(page, dictBodyPlaceholder, renderBody(reader, q, maxN, offset, relPath))
	io.WriteString(w, page)
}

// renderBody produces the main-column content: a keyword list (no query), an
// empty/no-match note, or the matched definitions rendered directly into the page
// (single DOM). The dict's own css/js is intentionally allowed to apply — server
// -rendered <script> tags execute, so interactive content (collapsibles) works
// and the theme cascade reaches the definitions. The per-dict <link>/<script src>
// tags are still deduped so they don't repeat once per entry.
func renderBody(reader *Reader, q string, maxN int, offset int, relPath string) string {
	if reader == nil {
		return `<p class="empty">Select a dictionary to begin.</p>`
	}
    if q == "" {
        var b strings.Builder
        b.WriteString(`<ul class="keywords">`)
        keys := reader.keywordsOffset(offset, maxN)
        for _, k := range keys {
            fmt.Fprintf(&b, `<li><a href="?path=%s&amp;q=%s">%s</a></li>`,
                pyQuote(relPath), pyQuote(k), html.EscapeString(k))
        }
        b.WriteString(`</ul>`)

        // pager
        total := len(reader.mdxEntries)
        hasPrev := offset > 0
        hasNext := offset+len(keys) < total

        if hasPrev || hasNext {
            b.WriteString(`<div class="pager">`)
            // jump to start
            if hasPrev {
                fmt.Fprintf(&b, `<a class="start" href="?path=%s&amp;offset=0&amp;max=%d">&laquo;</a>`, pyQuote(relPath), maxN)
            }
            if hasPrev {
                prev := offset - maxN
                if prev < 0 {
                    prev = 0
                }
                fmt.Fprintf(&b, `<a class="prev" href="?path=%s&amp;offset=%d&amp;max=%d">&#8249;</a>`, pyQuote(relPath), prev, maxN)
            }
            if hasNext {
                next := offset + maxN
                fmt.Fprintf(&b, `<a class="next" href="?path=%s&amp;offset=%d&amp;max=%d">&#8250;</a>`, pyQuote(relPath), next, maxN)
            }
            // jump to end
            if hasNext {
                last := total - (total % maxN)
                if last == total {
                    last = total - maxN
                }
                if last < 0 {
                    last = 0
                }
                fmt.Fprintf(&b, `<a class="end" href="?path=%s&amp;offset=%d&amp;max=%d">&raquo;</a>`, pyQuote(relPath), last, maxN)
            }
            // range label
            start := offset + 1
            end := offset + len(keys)
            if len(keys) == 0 {
                start = 0
            }
            fmt.Fprintf(&b, `<span class="range">%d–%d / %s</span>`, start, end, compactNum(total))
            b.WriteString(`</div>`)
        }

        return b.String()
    }
	defs := reader.search(q, maxN)
	for _, defi := range defs {
		reader.extractAssets(defi) // run on originals, before tags are stripped
	}
	cleaned, head := hoistHeadAssets(defs)
	if len(cleaned) == 0 {
		return `<p class="empty">No matches for “` + html.EscapeString(q) + `”.</p>`
	}
	return head + `<div class="content">` + strings.Join(cleaned, "\n") + `</div>`
}

// buildOptions renders <option> elements for every .mdx under dictDir,
// sorted case-insensitively by filename.
func buildOptions(selected string) string {
	var files []string
	_ = filepath.WalkDir(dictDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".mdx") {
			files = append(files, p)
		}
		return nil
	})
	sort.SliceStable(files, func(i, j int) bool {
		return strings.ToLower(filepath.Base(files[i])) < strings.ToLower(filepath.Base(files[j]))
	})

	var b strings.Builder
	for i, f := range files {
		rel, err := filepath.Rel(dictDir, f)
		if err != nil {
			rel = filepath.Base(f)
		}
		sel := ""
		if selected != "" && filepath.Clean(f) == filepath.Clean(selected) {
			sel = " selected"
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, `<option%s value="%s">%s</option>`, sel, filepath.ToSlash(rel), filepath.Base(f))
	}
	return b.String()
}

// compactNum formats large numbers like 120k, 1.2M
func compactNum(n int) string {
    if n >= 1_000_000 {
        return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
    }
    if n >= 1_000 {
        return fmt.Sprintf("%.1fk", float64(n)/1_000)
    }
    return strconv.Itoa(n)
}

// ── Config helpers ────────────────────────────────────────────────────────────
func getConfigPath() string {
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		if isFile(envPath) {
			return envPath
		}
		log.Printf("warning: CONFIG_PATH=%q is not a valid file, ignoring", envPath)
	}
	candidates := []func() (string, error){
		// exec dir
		func() (string, error) {
			exe, err := os.Executable()
			return filepath.Join(filepath.Dir(exe), configFileName), err
		},
		// ~/.mdict
		func() (string, error) {
			home, err := os.UserHomeDir()
			return filepath.Join(home, ".mdict", configFileName), err
		},
		// /etc/mdict
		func() (string, error) { return filepath.Join("/etc/mdict", configFileName), nil },
	}
	for _, fn := range candidates {
		if p, err := fn(); err == nil && isFile(p) {
			return p
		}
	}
	return configFileName
}
