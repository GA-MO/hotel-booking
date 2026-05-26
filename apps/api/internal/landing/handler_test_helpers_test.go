package landing

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// setChiURLParam attaches a chi route context with a single URL parameter
// to the request. Used to test handlers that call chi.URLParam without
// spinning up a full router.
func setChiURLParam(r *http.Request, key, value string) *http.Request {
	rctx, ok := r.Context().Value(chi.RouteCtxKey).(*chi.Context)
	if !ok {
		rctx = chi.NewRouteContext()
	}
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
