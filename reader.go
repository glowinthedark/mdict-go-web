// Copyright (C) 2024 glowinthedark
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

// Package main: mdict-go-web dictionary server
//
// reader.go exact/prefix/accent-fold lookup
// with @@@LINK following, HTML definition fix-ups (internal links, audio,
// sound://, file://, server-absolute refs, spx->wav), stylesheet substitution,
// and CSS-transitive extraction of .mdd resources into ASSET_ROOT. The raw
// MDict v1/v2/v3 block parsing is delegated to the vendored go-mdict package.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode"

	gomdict "github.com/glowinthedark/mdict-go-web/internal/gomdict"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	reHTMLRef    = regexp.MustCompile(`(?i)(?:src|href|xlink:href|poster)\s*=\s*["']([^"']+)["']`)
	reCSSURL     = regexp.MustCompile(`(?i)url\(\s*["']?([^"')]+?)["']?\s*\)`)
	reCSSImport  = regexp.MustCompile(`(?i)@import\s+(?:url\()?\s*["']([^"']+)["']`)
	reNonLocal   = regexp.MustCompile(`^(?:[a-zA-Z][\w+.\-]*:|//|#)`)
	reAbsRef     = regexp.MustCompile(`(?i)((?:src|href)=)(["'])(/+)`)
	reAudioTag   = regexp.MustCompile(`(?is)<audio\b[^>]*>(.*?)</audio>`)
	reAudioSrc   = regexp.MustCompile(`(?i)\bsrc\s*=\s*["']([^"']+)["']`)
	reSoundAnchr = regexp.MustCompile(`(?i)<a\b([^>]*?)href=(["'])sound://([^"']*)["']([^>]*)>`)
	reSourceTrk  = regexp.MustCompile(`(?i)<(?:source|track)\b[^>]*>`)
	reInternal   = regexp.MustCompile(`href=(["'])(?:entry://|bword://|[dx]:)([^"']*)["']`)
	reFileScheme = regexp.MustCompile(`(src|href)=(["'])file://`)
	reSoundSrc   = regexp.MustCompile(`(src|href)=(["'])sound://`)
	reSpxRef     = regexp.MustCompile(`(?i)((?:src|href)=["'][^"']*?\.spx)(["'])`)
	reStyleTag   = regexp.MustCompile("`\\d+`")
	// per-entry <link rel=stylesheet> / <script src=...> tags that belong in <head>
	reHoist = regexp.MustCompile(`(?is)<link\b[^>]*\brel\s*=\s*["']?stylesheet\b[^>]*>` +
		`|<script\b[^>]*\bsrc\s*=\s*["'][^"']*["'][^>]*>\s*</script>`)
)

const audioOnclick = "new Audio(this.href).play(); return false;"

// mddFile pairs an opened .mdd resource dictionary with its key entries.
type mddFile struct {
	md      *gomdict.Mdict
	entries []*gomdict.MDictKeywordEntry
}

// resHit locates a single resource inside one of the .mdd files.
type resHit struct {
	md    *gomdict.Mdict
	entry *gomdict.MDictKeywordEntry
}

// Reader is a single opened dictionary (one .mdx plus its companion .mdd files).
// It is safe for concurrent read use after openReader returns.
type Reader struct {
	filename   string
	name       string
	mdx        *gomdict.Mdict
	mdxEntries []*gomdict.MDictKeywordEntry
	mdxEnc     int
	stylesheet map[string][2]string
	substyle   bool
	audio      bool
	mdds       []mddFile

	resOnce sync.Once
	resMap  map[string]resHit
}

// openReader opens `filename` (an .mdx path) plus any companion .mdd files,
// including the FILE.mdd / FILE.1.mdd / FILE.N.mdd scheme.
func openReader(filename string) (*Reader, error) {
	mdx, err := gomdict.New(filename)
	if err != nil {
		return nil, err
	}
	if err := mdx.BuildIndex(); err != nil {
		return nil, err
	}
	entries, err := mdx.GetKeyWordEntries()
	if err != nil {
		return nil, err
	}
	// go-mdict leaves non-UTF16 MDX headwords as raw native-encoding bytes and
	// does not strip them; Normalize here so matching, @@@LINK following, listing and
	// resource lookup all operate on the same UTF-8, whitespace-trimmed form.
	enc := mdx.Encoding()
	for _, e := range entries {
		e.KeyWord = strings.TrimSpace(decodeEnc([]byte(e.KeyWord), enc))
	}
	r := &Reader{
		filename:   filename,
		mdx:        mdx,
		mdxEntries: entries,
		mdxEnc:     mdx.Encoding(),
		stylesheet: parseStylesheet(mdx.StyleSheet()),
		substyle:   true,
		audio:      true,
	}
	r.name = dictName(mdx, filename)

	noExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	candidates := []string{noExt + ".mdd", noExt + ".1.mdd"}
	for n := 2; isFile(fmt.Sprintf("%s.%d.mdd", noExt, n)); n++ {
		candidates = append(candidates, fmt.Sprintf("%s.%d.mdd", noExt, n))
	}
	for _, f := range candidates {
		if !isFile(f) {
			continue
		}
		md, err := gomdict.New(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open mdd %s: %v\n", f, err)
			continue
		}
		if err := md.BuildIndex(); err != nil {
			fmt.Fprintf(os.Stderr, "index mdd %s: %v\n", f, err)
			continue
		}
		ents, err := md.GetKeyWordEntries()
		if err != nil {
			fmt.Fprintf(os.Stderr, "entries mdd %s: %v\n", f, err)
			continue
		}
		r.mdds = append(r.mdds, mddFile{md: md, entries: ents})
	}
	dataCount := 0
	for _, m := range r.mdds {
		dataCount += len(m.entries)
	}
	fmt.Fprintf(os.Stderr, "Found %d mdd files with %d entries\n", len(r.mdds), dataCount)
	return r, nil
}

// blanking the MdxBuilder placeholder, falling back to the file stem.
func dictName(mdx *gomdict.Mdict, filename string) string {
	title := strings.TrimSpace(mdx.Title())
	if title == "Title (No HTML code allowed)" {
		title = ""
	}
	if title != "" {
		return title
	}
	base := filepath.Base(filename)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// ---- record decoding ----------------------------------------------------------

// decodeEnc converts bytes in the dictionary's native encoding to a UTF-8
// string.
// go-mdict already returns UTF-16 records decoded to UTF-8, so UTF-16/UTF-8 are
// pass-through; the CJK encodings are transcoded here.
func decodeEnc(raw []byte, enc int) string {
	switch enc {
	case gomdict.EncodingUtf16, gomdict.EncodingUtf8:
		return string(raw)
	case gomdict.EncodingGb18030, gomdict.ENCODING_GBK, gomdict.ENCODING_GB2312:
		out, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), raw)
		if err != nil {
			return string(raw)
		}
		return string(out)
	case gomdict.EncodingBig5:
		out, _, err := transform.Bytes(traditionalchinese.Big5.NewDecoder(), raw)
		if err != nil {
			return string(raw)
		}
		return string(out)
	default:
		return string(raw)
	}
}

func (r *Reader) decodeRecord(raw []byte) string { return decodeEnc(raw, r.mdxEnc) }

func (r *Reader) treatRecord(raw []byte) string {
	txt := r.decodeRecord(raw)
	txt = strings.Trim(txt, "\x00")
	if r.substyle && len(r.stylesheet) > 0 {
		txt = r.substituteStylesheet(txt)
	}
	return txt
}

func (r *Reader) substituteStylesheet(txt string) string {
	parts := reStyleTag.Split(txt, -1)
	tags := reStyleTag.FindAllString(txt, -1)
	var b strings.Builder
	b.WriteString(parts[0])
	for j := 1; j < len(parts); j++ {
		key := strings.Trim(tags[j-1], "`")
		style, ok := r.stylesheet[key]
		if !ok {
			continue
		}
		p := parts[j]
		if len(p) > 0 && p[len(p)-1] == '\n' {
			b.WriteString(style[0])
			b.WriteString(strings.TrimRightFunc(p, unicode.IsSpace))
			b.WriteString(style[1])
			b.WriteString("\r\n")
		} else {
			b.WriteString(style[0])
			b.WriteString(p)
			b.WriteString(style[1])
		}
	}
	return b.String()
}

func parseStylesheet(s string) map[string][2]string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	lines := regexp.MustCompile(`\r\n|\r|\n`).Split(s, -1)
	m := make(map[string][2]string)
	for i := 0; i+2 < len(lines); i += 3 {
		m[lines[i]] = [2]string{lines[i+1], lines[i+2]}
	}
	return m
}

// ---- lookup -------------------------------------------------------------------

type match struct {
	headword string
	raw      []byte
}

func (r *Reader) locate(e *gomdict.MDictKeywordEntry) []byte {
	data, err := r.mdx.LocateByKeywordEntry(e)
	if err != nil {
		fmt.Fprintf(os.Stderr, "locate %q: %v\n", e.KeyWord, err)
		return nil
	}
	return data
}

// lookup returns records whose headword matches `word`: all exact matches, or —
// if none — all accent/case-insensitive (folded) matches.
func (r *Reader) lookup(word string) []match {
	out := r.collect(func(hw string) bool { return hw == word })
	if len(out) == 0 {
		k := fold(word)
		out = r.collect(func(hw string) bool { return fold(hw) == k })
	}
	return out
}

func (r *Reader) collect(pred func(string) bool) []match {
	var out []match
	for _, e := range r.mdxEntries {
		if pred(e.KeyWord) {
			out = append(out, match{headword: e.KeyWord, raw: r.locate(e)})
		}
	}
	return out
}

// ALL exact matches, else the first
// `limit` prefix matches; accent-fold pass only when the raw pass is empty.
func (r *Reader) lookupPrefix(word string, limit int) []match {
	scan := func(useFold bool) []int {
		key := word
		if useFold {
			key = fold(word)
		}
		var exact, prefix []int
		for i, e := range r.mdxEntries {
			hw := e.KeyWord
			if useFold {
				hw = fold(hw)
			}
			if hw == key {
				exact = append(exact, i)
			} else if len(prefix) < limit && strings.HasPrefix(hw, key) {
				prefix = append(prefix, i)
			}
		}
		if len(exact) > 0 {
			return exact
		}
		return prefix
	}
	idxs := scan(false)
	if len(idxs) == 0 {
		idxs = scan(true)
	}
	if len(idxs) > limit {
		idxs = idxs[:limit]
	}
	out := make([]match, 0, len(idxs))
	for _, i := range idxs {
		e := r.mdxEntries[i]
		out = append(out, match{headword: e.KeyWord, raw: r.locate(e)})
	}
	return out
}

// strip combining marks for accent/case-insensitive
// matching.
func fold(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, ru := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, ru) {
			continue
		}
		b.WriteRune(ru)
	}
	return b.String()
}

// ---- rendering ----------------------------------------------------------------

// render turns one raw record into [HTML defi], following an @@@LINK redirect
// (exact, cycle-guarded).
func (r *Reader) render(raw []byte, seen map[string]bool) []string {
	defi := strings.TrimSpace(r.treatRecord(raw))
	if strings.HasPrefix(defi, "@@@LINK=") {
		target := strings.Trim(strings.TrimSpace(defi[len("@@@LINK="):]), "\x00")
		if target != "" && !seen[target] {
			return r.define(target, seen)
		}
		return nil
	}
	return []string{r.fixDefi(defi)}
}

// define returns HTML definitions for the exact headword `word` (+ homographs),
// following @@@LINK.
func (r *Reader) define(word string, seen map[string]bool) []string {
	if seen == nil {
		seen = map[string]bool{}
	}
	seen[word] = true
	var out []string
	for _, m := range r.lookup(word) {
		out = append(out, r.render(m.raw, seen)...)
	}
	return out
}

// search returns HTML definitions for headwords matching `word` (exact, else
// prefix), capped at `limit`.
func (r *Reader) search(word string, limit int) []string {
	var out []string
	for _, m := range r.lookupPrefix(word, limit) {
		out = append(out, r.render(m.raw, map[string]bool{m.headword: true})...)
		if len(out) >= limit {
			break
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ---- HTML fix-ups -------------------------------------------------------------

// rewrite internal links to in-dictionary
// queries, normalize file://, in-page audio play, sound://, spx->wav, and
// server-absolute refs.
func (r *Reader) fixDefi(defi string) string {
	defi = reInternal.ReplaceAllStringFunc(defi, r.internalLinkRepl)
	defi = reFileScheme.ReplaceAllString(defi, "$1=$2")

	if r.audio {
		defi = reAudioTag.ReplaceAllStringFunc(defi, audioTagRepl)
		defi = reSoundAnchr.ReplaceAllStringFunc(defi, soundAnchorRepl)
		defi = reSoundSrc.ReplaceAllString(defi, "$1=$2")
		defi = reSpxRef.ReplaceAllString(defi, "$1.wav$2")
	}

	defi = reAbsRef.ReplaceAllStringFunc(defi, absRefRepl)
	return defi
}

func (r *Reader) internalLinkRepl(s string) string {
	m := reInternal.FindStringSubmatch(s)
	q, word := m[1], m[2]
	// ?path= is interpreted relative to dictDir on the server (handlePage).
	dictPathRel, err := filepath.Rel(dictDir, r.filename)
	if err != nil {
		dictPathRel = r.filename // not under dictDir: fall back to absolute
	}
	return "href=" + q + "?path=" + pyQuote(filepath.ToSlash(dictPathRel)) + "&amp;q=" + pyQuote(word) + q
}

func audioTagRepl(s string) string {
	m := reAudioTag.FindStringSubmatch(s)
	inner := m[1]
	srcm := reAudioSrc.FindStringSubmatch(s)
	if srcm == nil {
		return inner
	}
	src := srcm[1]
	if strings.HasPrefix(strings.ToLower(src), "sound://") {
		src = src[len("sound://"):]
	}
	label := strings.TrimSpace(reSourceTrk.ReplaceAllString(inner, ""))
	if label == "" {
		label = "🔊"
	}
	return fmt.Sprintf(`<a onclick="%s" href="%s">%s</a>`, audioOnclick, src, label)
}

func soundAnchorRepl(s string) string {
	m := reSoundAnchr.FindStringSubmatch(s)
	pre, q, p, post := m[1], m[2], m[3], m[4]
	onclick := ""
	if !strings.Contains(strings.ToLower(pre+post), "onclick") {
		onclick = fmt.Sprintf(`onclick="%s" `, audioOnclick)
	}
	return "<a" + pre + onclick + "href=" + q + p + q + post + ">"
}

func absRefRepl(s string) string {
	m := reAbsRef.FindStringSubmatch(s)
	pre, q, slashes := m[1], m[2], m[3]
	if len(slashes) == 1 {
		return pre + q // strip the single leading slash -> relative ref
	}
	return pre + q + slashes // protocol-relative (//host) or multi-slash: untouched
}

// quote with the default safe="/": unreserved
// characters and '/' pass through, everything else is percent-encoded.
func pyQuote(s string) string {
	const upper = "0123456789ABCDEF"
	safe := func(c byte) bool {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			return true
		case c == '_' || c == '.' || c == '-' || c == '~' || c == '/':
			return true
		}
		return false
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if safe(c) {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(upper[c>>4])
			b.WriteByte(upper[c&0xF])
		}
	}
	return b.String()
}

// ---- resource extraction ------------------------------------------------------

func (r *Reader) resourceIndex() map[string]resHit {
	r.resOnce.Do(func() {
		m := make(map[string]resHit)
		for i := range r.mdds {
			md := &r.mdds[i]
			for _, e := range md.entries {
				key := strings.ReplaceAll(strings.TrimLeft(e.KeyWord, "\\/"), "\\", "/")
				norm := strings.ToLower(key)
				if _, ok := m[norm]; !ok {
					m[norm] = resHit{md: md.md, entry: e}
				}
			}
		}
		r.resMap = m
	})
	return r.resMap
}

func refIsLocal(ref string) bool {
	return !reNonLocal.MatchString(ref)
}

func stripFragQuery(ref string) string {
	ref = strings.SplitN(ref, "#", 2)[0]
	ref = strings.SplitN(ref, "?", 2)[0]
	return ref
}

// extractAssets extracts every local resource referenced by `html` (transitively
// via CSS) to ASSET_ROOT
func (r *Reader) extractAssets(html string) {
	index := r.resourceIndex()
	if len(index) == 0 {
		return
	}
	seen := map[string]bool{}
	var queue []string
	for _, m := range reHTMLRef.FindAllStringSubmatch(html, -1) {
		if refIsLocal(m[1]) {
			queue = append(queue, m[1])
		}
	}
	for len(queue) > 0 {
		ref := queue[len(queue)-1]
		queue = queue[:len(queue)-1]

		norm := strings.TrimLeft(path.Clean(stripFragQuery(ref)), "/")
		if norm == "" || norm == "." || strings.HasPrefix(norm, "..") || seen[norm] {
			continue
		}
		seen[norm] = true

		hit, ok := index[strings.ToLower(norm)]
		if !ok {
			// "<name>.spx.wav" has no MDD entry: extract "<name>.spx" and transcode once.
			if strings.HasSuffix(norm, ".spx.wav") {
				r.transcodeSpx(norm[:len(norm)-4], norm, index)
			}
			continue
		}
		dest := filepath.Join(assetRoot, filepath.FromSlash(norm))
		var data []byte
		// regenerate when missing OR empty: never trust a 0-byte cached file
		if st, err := os.Stat(dest); err != nil || st.Size() == 0 {
			data = locateRes(hit)
			if len(data) > 0 {
				_ = os.MkdirAll(filepath.Dir(dest), 0o755)
				if err := os.WriteFile(dest, data, 0o644); err != nil {
					fmt.Fprintf(os.Stderr, "write asset %s: %v\n", dest, err)
				}
			}
		}
		// CSS pulls in fonts/sprites/@imports by relative url -- follow them.
		if strings.HasSuffix(norm, ".css") {
			if data == nil {
				data, _ = os.ReadFile(dest)
			}
			css := string(data)
			base := path.Dir(norm)
			for _, m := range reCSSURL.FindAllStringSubmatch(css, -1) {
				if refIsLocal(m[1]) {
					queue = append(queue, path.Join(base, stripFragQuery(m[1])))
				}
			}
			for _, m := range reCSSImport.FindAllStringSubmatch(css, -1) {
				if refIsLocal(m[1]) {
					queue = append(queue, path.Join(base, stripFragQuery(m[1])))
				}
			}
		}
	}
}

func locateRes(h resHit) []byte {
	data, err := h.md.LocateByKeywordEntry(h.entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "locate resource %q: %v\n", h.entry.KeyWord, err)
		return nil
	}
	return data
}

// transcodeSpx extracts the Speex resource `srcNorm` and transcodes it to
// `destNorm` (WAV) via speexdec, once.
func (r *Reader) transcodeSpx(srcNorm, destNorm string, index map[string]resHit) {
	hit, ok := index[strings.ToLower(srcNorm)]
	dest := filepath.Join(assetRoot, filepath.FromSlash(destNorm))
	// skip only when a NON-empty transcode already exists (0-byte = retry)
	if !ok {
		return
	}
	if st, err := os.Stat(dest); err == nil && st.Size() > 0 {
		return
	}
	src := filepath.Join(assetRoot, filepath.FromSlash(srcNorm))
	if st, err := os.Stat(src); err != nil || st.Size() == 0 {
		data := locateRes(hit)
		if len(data) == 0 {
			fmt.Fprintf(os.Stderr, "empty source resource %s, skipping transcode\n", srcNorm)
			return
		}
		_ = os.MkdirAll(filepath.Dir(src), 0o755)
		if err := os.WriteFile(src, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write spx source %s: %v\n", src, err)
			return
		}
	}
	// speexDecPath = speexdec binary used to transcode Speex (.spx) audio to browser-playable WAV.
	cmd := exec.Command(speexDecPath, src, dest)
	if err := cmd.Run(); err != nil {
		// don't leave a 0-byte transcode behind: it would be served empty AND cached
		if st, e := os.Stat(dest); e == nil && st.Size() == 0 {
			_ = os.Remove(dest)
		}
		fmt.Fprintf(os.Stderr, "spx transcode failed for %s: %v\n", srcNorm, err)
	}
}

// hoistHeadAssets strips per-entry <link rel=stylesheet> and <script src> tags
// from each definition and returns the cleaned definitions plus the deduped set
// of tags to inject once into <head> (instead of repeating them per entry in the
// body). Asset extraction must run on the originals before calling this.
func hoistHeadAssets(defs []string) (cleaned []string, head string) {
	seen := map[string]bool{}
	var b strings.Builder
	cleaned = make([]string, 0, len(defs))
	for _, d := range defs {
		d = reHoist.ReplaceAllStringFunc(d, func(tag string) string {
			key := strings.Join(strings.Fields(tag), " ") // normalize whitespace for dedup
			if !seen[key] {
				seen[key] = true
				b.WriteString(tag)
				b.WriteByte('\n')
			}
			return ""
		})
		cleaned = append(cleaned, d)
	}
	return cleaned, b.String()
}

// keywords returns the first n headwords, for the bare key listing.
func (r *Reader) keywords(n int) []string {
	if n > len(r.mdxEntries) {
		n = len(r.mdxEntries)
	}
	out := make([]string, 0, n)
	for _, e := range r.mdxEntries[:n] {
		out = append(out, e.KeyWord)
	}
	return out
}

// ---- small helpers ------------------------------------------------------------

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
