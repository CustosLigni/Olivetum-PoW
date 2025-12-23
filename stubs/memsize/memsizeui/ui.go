package memsizeui

import "net/http"

// Handler is a no-op implementation that satisfies the interface expected by
// the debug package. The real memsize UI is only useful for manual inspection,
// which we can safely skip in this environment.
type Handler struct{}

func (h *Handler) Add(string, interface{}) {}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
