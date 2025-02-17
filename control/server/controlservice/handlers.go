package controlservice

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/caldog20/calnet/control/server/store"
	"github.com/caldog20/calnet/pkg/controlapi"
	"github.com/caldog20/calnet/pkg/keys"
)

// func decrypt[T any](data []byte, serverKey keys.PrivateKey, senderKey keys.PublicKey) (T, bool) {
// 	var t T
//
// }

func (c *Control) handleControlKey(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := &controlapi.ControlKey{
		PublicKey: c.publicKey,
	}

	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Printf("error encoding server key response: %s", err)
	}
}

func (c *Control) handleLogin(w http.ResponseWriter, r *http.Request) {
	controlKeyStr := r.Header.Get("x-control-key")
	controlKey := keys.PublicKey{}
	err := controlKey.DecodeFromString(controlKeyStr)
	if err != nil {
		http.Error(w, "error decoding control key", http.StatusBadRequest)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error reading request body", http.StatusBadRequest)
		return
	}

	if !c.disableControlNacl {
		var ok bool
		data, ok = c.privateKey.DecryptBox(data, controlKey)
		if !ok {
			http.Error(w, "error decrypting message", http.StatusBadRequest)
			return
		}
	}

	login := controlapi.LoginRequest{}
	err = json.Unmarshal(data, &login)
	if err != nil {
		http.Error(w, "error decoding login request", http.StatusBadRequest)
		return
	}

	log.Printf("processing login for node key: %s", login.NodeKey.EncodeToString())
	n, err := c.store.GetNodeByKey(login.NodeKey)
	if err != nil {
		if !errors.Is(err, store.ErrNodeNotFound) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		} else {
			// Node not found, try to create
			if login.ProvisionKey != "please" {
				http.Error(w, "invalid provision key to register", http.StatusUnauthorized)
				return
			}
			n, err = c.createNode(login.NodeKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	} else {
		if n.IsExpired() {
			http.Error(w, "node is expired", http.StatusUnauthorized)
			return
		}
	}

	resp := &controlapi.LoginResponse{
		LoggedIn: true,
	}

	data, err = json.Marshal(resp)
	if err != nil {
		http.Error(w, "error marshalling response", http.StatusInternalServerError)
		return
	}

	if !c.disableControlNacl {
		data = c.privateKey.EncryptBox(data, controlKey)
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	if err != nil {
		log.Printf("error writing login response: %s", err)
	}
	c.notifyAll()
}

func (c *Control) handlePoll(w http.ResponseWriter, r *http.Request) {
	controlKeyStr := r.Header.Get("x-control-key")
	controlKey := keys.PublicKey{}
	err := controlKey.DecodeFromString(controlKeyStr)
	if err != nil {
		http.Error(w, "error decoding control key", http.StatusBadRequest)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error reading request body", http.StatusBadRequest)
		return
	}

	if !c.disableControlNacl {
		var ok bool
		data, ok = c.privateKey.DecryptBox(data, controlKey)
		if !ok {
			http.Error(w, "error decrypting message", http.StatusBadRequest)
			return
		}
	}

	pollRequest := controlapi.PollRequest{}
	err = json.Unmarshal(data, &pollRequest)
	if err != nil {
		http.Error(w, "error decoding poll request", http.StatusBadRequest)
		return
	}

	n, err := c.store.GetNodeByKey(pollRequest.NodeKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := &controlapi.PollResponse{}
	writeResponse := func() {
		data, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "error marshalling poll response", http.StatusInternalServerError)
			return
		}
		if !c.disableControlNacl {
			data = c.privateKey.EncryptBox(data, controlKey)
		}
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(data)
		if err != nil {
			log.Printf("error writing poll response: %s", err)
			return
		}
	}

	if n.IsExpired() {
		resp.KeyExpired = true
		writeResponse()
		return
	}

	timeout := time.NewTimer(time.Second * 50)

	notifyCh := c.getNodePollChan(n.ID)

	select {
	case <-r.Context().Done():
		return
	case <-timeout.C:
		w.WriteHeader(http.StatusNoContent)
		return
	case <-notifyCh:
		resp, err = c.getUpdate(n)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeResponse()
		return
	}
}
