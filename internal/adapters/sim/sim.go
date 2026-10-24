// Package sim is the HTTP adapter for the 3D concept simulator. It serves the
// embedded static front end: the /sim catalog, the per-concept pages and the
// assets under /static/sim/ (engine, concepts, vendored libraries). It holds no
// domain logic; the simulations run entirely in the browser.
package sim

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"time"
)

//go:embed static
var embedded embed.FS

const assetPrefix = "/static/sim/"

// segment limits the {topic} and {id} path values to what registry ids use.
var segment = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// Handler serves the simulator front end.
type Handler struct {
	files fs.FS
	// etags maps a file name under static/ to its ETag. It doubles as the
	// allowlist of servable files, so directories and unknown names are 404.
	etags map[string]string
}

// New indexes the embedded files and computes their ETags once.
func New() (*Handler, error) {
	files, err := fs.Sub(embedded, "static")
	if err != nil {
		return nil, err
	}
	etags := map[string]string{}
	err = fs.WalkDir(files, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		etags[name] = `"` + hex.EncodeToString(sum[:8]) + `"`
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Handler{files: files, etags: etags}, nil
}

// Mount registers the simulator routes on mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /sim", func(w http.ResponseWriter, r *http.Request) {
		h.serve(w, r, "index.html")
	})
	mux.HandleFunc("GET /sim/{topic}/{id}", h.concept)
	mux.HandleFunc("GET "+assetPrefix, h.asset)
}

// concept serves the one page shell for every concept; the browser reads the
// topic and id from the URL and the registry decides what exists.
func (h *Handler) concept(w http.ResponseWriter, r *http.Request) {
	if !segment.MatchString(r.PathValue("topic")) || !segment.MatchString(r.PathValue("id")) {
		http.NotFound(w, r)
		return
	}
	h.serve(w, r, "concept.html")
}

func (h *Handler) asset(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, strings.TrimPrefix(r.URL.Path, assetPrefix))
}

// serve writes one embedded file. no-cache plus an ETag makes browsers
// revalidate, so a rebuilt binary is picked up while unchanged files return 304.
func (h *Handler) serve(w http.ResponseWriter, r *http.Request, name string) {
	etag, ok := h.etags[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := h.files.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	// The server's WriteTimeout (10s) would cut off large files such as
	// three.core.js (~1.5 MB) on slow links. Ignore the error: recorders and
	// wrappers without deadline support just keep the server default.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(60 * time.Second))
	http.ServeContent(w, r, name, time.Time{}, rs)
}
