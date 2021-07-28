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
	keySet             jwk.Set
	fetchTimeout       time.Duration
	keyUpdateSemaphore *semaphore.Weighted
	keyUpdateChannel   chan error
	keyUpdateCount     int
	keyUpdateLimiter   ratelimit.Limiter
}

func newKeyHandler(jwksUri string, fetchTimeout time.Duration, keyUpdateRPS uint) (*keyHandler, error) {
	h := &keyHandler{
		jwksURI:            jwksUri,
		fetchTimeout:       fetchTimeout,
		keyUpdateSemaphore: semaphore.NewWeighted(int64(1)),
		keyUpdateChannel:   make(chan error),
		keyUpdateLimiter:   ratelimit.New(int(keyUpdateRPS)),
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
	h.keyUpdateCount = h.keyUpdateCount + 1
	h.Unlock()

	return nil
}

func (h *keyHandler) waitForUpdateKeySet() error {
	ok := h.keyUpdateSemaphore.TryAcquire(1)
	if ok {
		defer h.keyUpdateSemaphore.Release(1)
		_ = h.keyUpdateLimiter.Take()
		err := h.updateKeySet()

		for {
			select {
			case h.keyUpdateChannel <- err:
			default:
				return err
			}
		}
	}

	return <-h.keyUpdateChannel
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
		err := h.waitForUpdateKeySet()
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
