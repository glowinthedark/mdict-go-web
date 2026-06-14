// MDict dictionary self-contained HTTP server.
//
// a single binary serves the search page, handles ?q / ?path / ?max queries,
// and serves .mdd resources extracted on demand into ASSET_ROOT.
//
// Configuration (env var -> hard-coded fallback):
//
//	DICT_DIR               ~/Dictionaries      dir of .mdx/.mdd files
//	MDICT_TEMP_ASSETS_DIR  ~/.mdict/res    extracted-asset root
//	SERVER_IP              127.0.0.1
//	SERVER_PORT            8808
//	SPEEXDEC               /usr/local/bin/speexdec
package main

import (
	_ "embed"
	"fmt"
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

const (
	maxItemsDefault        = 42
	configFileName         = "config.toml"
	dictNamePlaceholder    = "$$${{{DICT_NAME}}}"
	dictOptionsPlaceholder = "$$${{{DICT_OPTIONS}}}"
)

// Resolved configuration, set in main.
var (
	dictDir     string
	assetRoot   string
	defaultPath string
)

// Reader cache: the server is long-lived
// so each dictionary is parsed once and reused across requests.
var (
	readerCacheMu sync.Mutex
	readerCache   = map[string]*Reader{}
)

func getReader(path string) (*Reader, error) {
	readerCacheMu.Lock()
	defer readerCacheMu.Unlock()
	if r, ok := readerCache[path]; ok {
		return r, nil
	}
	r, err := openReader(path)
	if err != nil {
		return nil, err
	}
	readerCache[path] = r
	return r, nil
}

func main() {
	LoadConfig(getConfigPath())
	dictDir = mustAbs(expandTilde(getConf("DICT_DIR", "~/Dictionaries")))
	assetRoot = expandTilde(getConf("MDICT_TEMP_ASSETS_DIR", "~/.mdict/res"))
	defaultPath = expandTilde(getConf("DEFAULT_DICT", ""))

	if !dirExists(assetRoot) {
		fmt.Fprintf(os.Stderr, "Creating temporary asset directory %s for extracted .mdd resources...\n", assetRoot)
		if err := os.MkdirAll(assetRoot, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "could not create asset dir %s: %v\n", assetRoot, err)
		}
	}

	ip := getConf("SERVER_IP", "127.0.0.1")
	port := getConf("SERVER_PORT", "8808")
	addr := ip + ":" + port

	fmt.Printf("DICT_DIR: %s\n", dictDir)
	fmt.Printf("MDICT_TEMP_ASSETS_DIR: %s\n", assetRoot)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)

	// Bind before opening the browser so the URL is live when it loads.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not bind %s: %v\n", addr, err)
		os.Exit(1)
	}
	url := fmt.Sprintf("http://%s/", addr)
	fmt.Printf("Starting MDict server: %s\n", url)

	if getConf("NO_BROWSER", "") == "" {
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintf(os.Stderr, "could not open browser: %v\n", err)
		}
	}

	if err := http.Serve(ln, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

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

// serveAsset serves an extracted resource from ASSET_ROOT. Relative refs like
// "img/x.png" in a definition resolve here
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
		fmt.Fprintf(w, `<h3 style='color:red'>Not found: DICT_DIR=%s</h3>
		`+"\n"+`<p>The path to the directory with .mdx/.mdd files can be set via:
		<ol>
		<li><tt>DICT_DIR</tt> in <tt>config.toml</tt> 
		<li>As environment variable <tt>DICT_DIR</tt>
		<li>Passed from command line e.g. <pre>DICT_DIR=/path/to/dict/dir ./mdict-server</pre>
		</ol>
		`, dictDir)
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
	relPath := qv.Get("path")
	if relPath == "" {
		relPath = defaultPath
	}
	absPathIn := mustAbs(filepath.Join(dictDir, relPath))

	html := templateHTML
	var reader *Reader
	if isFile(absPathIn) {
		rd, err := getReader(absPathIn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open dictionary %s: %v\n", absPathIn, err)
			html = strings.ReplaceAll(html, dictNamePlaceholder, "")
		} else {
			reader = rd
			html = strings.ReplaceAll(html, dictNamePlaceholder, rd.name)
		}
	} else {
		html = strings.ReplaceAll(html, dictNamePlaceholder, "")
	}

	html = strings.ReplaceAll(html, dictOptionsPlaceholder, buildOptions(absPathIn))

	io.WriteString(w, html+"\n")

	if reader != nil {
		if q != "" {
			for _, defi := range reader.search(q, maxN) {
				reader.extractAssets(defi)
				io.WriteString(w, defi+"\n")
			}
		} else {
			io.WriteString(w, "<pre>\n")
			io.WriteString(w, strings.Join(reader.keywords(maxN), "\n"))
			io.WriteString(w, "\n</pre>\n")
		}
	}
	io.WriteString(w, "</div>\n</div>\n</body>\n</html>\n")
}

// buildOptions renders the <option> list for every .mdx under DICT_DIR, sorted
// by filename (case-insensitive), marking the selected one.
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
	// Stable sort by case-folded basename, keep WalkDir's deterministic order.
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

// ---- config helpers -----------------------------------------------------------

func expandTilde(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
		}
	}
	return p
}

func mustAbs(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func getConfigPath() string {
	// env var override (highest priority)
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		if isFile(envPath) {
			return envPath
		}
		log.Printf("warning: CONFIG_PATH=%q is not a valid file, ignoring", envPath)
	}

	// ordered candidate locations
	candidates := []func() (string, error){
		func() (string, error) {
			exe, err := os.Executable()
			return filepath.Join(filepath.Dir(exe), configFileName), err
		},
		func() (string, error) {
			home, err := os.UserHomeDir()
			return filepath.Join(home, ".mdict", configFileName), err
		},
		func() (string, error) {
			return filepath.Join("/etc/mdict", configFileName), nil
		},
	}

	for _, candidate := range candidates {
		if path, err := candidate(); err == nil && isFile(path) {
			return path
		}
	}

	//  cwd/configFileName
	return configFileName
}
