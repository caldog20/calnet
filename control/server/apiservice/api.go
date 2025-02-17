package apiservice

import (
	"net/http"

	"github.com/caldog20/calnet/control/server/internal/store"
)

type RestAPI struct{
    disableAuth bool
    store store.Store
}

func New(store store.Store) *RestAPI {
    return &RestAPI{store: store}
}

func (r *RestAPI) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("GET /api/v1/nodes", r.handleGetNodes)
    mux.HandleFunc("GET /api/v1/node/{id}", r.handleGetNodeByID)
}
