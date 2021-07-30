package oidcdiscovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/jwk"
	"go.uber.org/ratelimit"
	"golang.org/x/sync/semaphore"
)

type keyHandler struct {
	sync.RWMutex
	jwksURI            string
	disableKeyID       bool
	keySet             jwk.Set
	fetchTimeout       time.Duration
	keyUpdateSemaphore *semaphore.Weighted
	keyUpdateChannel   chan keyUpdate
	keyUpdateCount     int
	keyUpdateLimiter   ratelimit.Limiter
}

type keyUpdate struct {
	keySet jwk.Set
	err    error
}

func newKeyHandler(jwksUri string, fetchTimeout time.Duration, keyUpdateRPS uint, disableKeyID bool) (*keyHandler, error) {
	h := &keyHandler{
		jwksURI:            jwksUri,
		disableKeyID:       disableKeyID,
		fetchTimeout:       fetchTimeout,
		keyUpdateSemaphore: semaphore.NewWeighted(int64(1)),
		keyUpdateChannel:   make(chan keyUpdate),
		keyUpdateLimiter:   ratelimit.New(int(keyUpdateRPS)),
	}

	_, err := h.updateKeySet()
	if err != nil {
		return nil, err
	}

	return h, nil
}

func (h *keyHandler) updateKeySet() (jwk.Set, error) {
	ctx, cancel := context.WithTimeout(context.Background(), h.fetchTimeout)
	defer cancel()
	keySet, err := jwk.Fetch(ctx, h.jwksURI)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch keys from %q: %v", h.jwksURI, err)
	}

	if h.disableKeyID && keySet.Len() != 1 {
		return nil, fmt.Errorf("keyID is disabled, but received a keySet with more than one key: %d", keySet.Len())
	}

	h.Lock()
	h.keySet = keySet
	h.keyUpdateCount = h.keyUpdateCount + 1
	h.Unlock()

	return keySet, nil
}

func (h *keyHandler) waitForUpdateKeySet() (jwk.Set, error) {
	ok := h.keyUpdateSemaphore.TryAcquire(1)
	if ok {
		defer h.keyUpdateSemaphore.Release(1)
		_ = h.keyUpdateLimiter.Take()
		keySet, err := h.updateKeySet()

		k := keyUpdate{
			keySet,
			err,
		}

		for {
			select {
			case h.keyUpdateChannel <- k:
			default:
				return keySet, err
			}
		}
	}

	k := <-h.keyUpdateChannel
	return k.keySet, k.err
}

func (h *keyHandler) waitForUpdateKey() (jwk.Key, error) {
	keySet, err := h.waitForUpdateKeySet()
	if err != nil {
		return nil, err
	}

	key, found := keySet.Get(0)
	if !found {
		return nil, fmt.Errorf("no key found")
	}

	return key, nil
}

func (h *keyHandler) getKey(keyID string) (jwk.Key, error) {
	if h.disableKeyID {
		return h.getDefaultKey()
	}

	return h.getByKeyID(keyID)

}

func (h *keyHandler) getKeySet() jwk.Set {
	h.RLock()
	defer h.RUnlock()
	return h.keySet
}

func (h *keyHandler) getByKeyID(keyID string) (jwk.Key, error) {
	keySet := h.getKeySet()

	key, found := keySet.LookupKeyID(keyID)

	if !found {
		updatedKeySet, err := h.waitForUpdateKeySet()
		if err != nil {
			return nil, fmt.Errorf("unable to update key set for key %q: %v", keyID, err)
		}

		updatedKey, found := updatedKeySet.LookupKeyID(keyID)
		if !found {
			return nil, fmt.Errorf("unable to find key %q", keyID)
		}

		return updatedKey, nil
	}

	return key, nil
}

func (h *keyHandler) getDefaultKey() (jwk.Key, error) {
	keySet := h.getKeySet()

	key, found := keySet.Get(0)
	if !found {
		return nil, fmt.Errorf("no key found")
	}

	return key, nil
}
