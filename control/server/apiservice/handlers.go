package apiservice

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/caldog20/calnet/control/server/store"
)

type JSONError struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

func writeJSONError(w http.ResponseWriter, err error, code int) {
	werr := json.NewEncoder(w).Encode(JSONError{
		Error: err.Error(),
		Code:  code,
	})
	if werr != nil {
		log.Printf("error writing json error %s", werr)
	}
}

func (r *RestAPI) handleGetNodes(w http.ResponseWriter, req *http.Request) {
	nodes, err := r.store.GetNodes()
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}

	nodesResp := Nodes{}
	for _, n := range nodes {
		nodesResp.Nodes = append(nodesResp.Nodes, Node{
			ID:        n.ID,
			NodeKey:   n.NodeKey,
			KeyExpiry: n.KeyExpiry,
			IP:        n.IP,
			NetPrefix: n.Prefix,
			LastSeen:  n.LastConnected,
			CreatedAt: n.CreatedAt,
			UpdatedAt: n.UpdatedAt,
			User:      n.User,
			Disabled:  n.Disabled,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(nodesResp)
	if err != nil {
		log.Println("handleGetNodes: error encoding json response:", err)
	}
}

func (r *RestAPI) handleGetNodeByID(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	nodeID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		writeJSONError(w, errors.New("error parsing node id"), http.StatusBadRequest)
		return
	}

	log.Printf("getting node by id %d", nodeID)

	n, err := r.store.GetNodeByID(nodeID)
	if err != nil {
		if errors.Is(err, store.ErrNodeNotFound) {
			writeJSONError(w, err, http.StatusNotFound)
		} else {
			writeJSONError(w, err, http.StatusInternalServerError)
		}
		return
	}

	nodeResp := Node{
		ID:        n.ID,
		NodeKey:   n.NodeKey,
		KeyExpiry: n.KeyExpiry,
		IP:        n.IP,
		NetPrefix: n.Prefix,
		LastSeen:  n.LastConnected,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
		User:      n.User,
		Disabled:  n.Disabled,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(nodeResp)
	if err != nil {
		log.Println("handleGetNodeByID: error encoding json response:", err)
	}
}
