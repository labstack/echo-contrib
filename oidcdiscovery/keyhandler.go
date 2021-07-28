package oidcdiscovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/jwk"
)

type keyHandler struct {
	sync.RWMutex
	jwksURI      string
	keySet       jwk.Set
	fetchTimeout time.Duration
}

func newKeyHandler(jwksUri string, fetchTimeout time.Duration) (*keyHandler, error) {
	h := &keyHandler{
		jwksURI:      jwksUri,
		fetchTimeout: fetchTimeout,
	}

	err := h.updateKeySet()
	if err != nil {
		return nil, err
	}

	return h, nil
}

func (h *keyHandler) updateKeySet() error {
	ctx, cancel := context.WithTimeout(context.Background(), h.fetchTimeout)
	defer cancel()
	keySet, err := jwk.Fetch(ctx, h.jwksURI)
	if err != nil {
		return fmt.Errorf("unable to fetch keys from %q: %v", h.jwksURI, err)
	}

	h.Lock()
	h.keySet = keySet
	h.Unlock()

	return nil
}

func (h *keyHandler) getKeySet() jwk.Set {
	h.RLock()
	defer h.RUnlock()
	return h.keySet
}

func (h *keyHandler) getByKeyID(keyID string, retry bool) (jwk.Key, error) {
	keySet := h.getKeySet()
	key, found := keySet.LookupKeyID(keyID)

	if !found && !retry {
		err := h.updateKeySet()
		if err != nil {
			return nil, fmt.Errorf("unable to update key set for key %q: %v", keyID, err)
		}

		return h.getByKeyID(keyID, true)
	}

	if !found && retry {
		return nil, fmt.Errorf("unable to find key %q", keyID)
	}

	return key, nil
}
