package oidcdiscovery

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xenitab/dispans/server"
)

func TestNewKeyHandler(t *testing.T) {
	op := server.NewTesting(t)
	issuer := op.GetURL(t)
	discoveryUri := getDiscoveryUriFromIssuer(issuer)
	jwksUri, err := getJwksUriFromDiscoveryUri(discoveryUri, 10*time.Millisecond)
	require.NoError(t, err)

	keyHandler, err := newKeyHandler(jwksUri, 10*time.Millisecond, 100)
	require.NoError(t, err)

	keySet1 := keyHandler.getKeySet()
	require.Equal(t, 1, keySet1.Len())

	expectedKey1, ok := keySet1.Get(0)
	require.True(t, ok)

	token1 := op.GetToken(t)
	keyID1, err := getKeyIDFromTokenString(token1.AccessToken)
	require.NoError(t, err)

	// Test valid key id
	key1, err := keyHandler.getByKeyID(keyID1)
	require.NoError(t, err)
	require.Equal(t, expectedKey1, key1)

	// Test invalid key id
	_, err = keyHandler.getByKeyID("foo")
	require.Error(t, err)

	// Test with rotated keys
	op.RotateKeys(t)

	token2 := op.GetToken(t)
	keyID2, err := getKeyIDFromTokenString(token2.AccessToken)
	require.NoError(t, err)

	key2, err := keyHandler.getByKeyID(keyID2)
	require.NoError(t, err)

	keySet2 := keyHandler.getKeySet()
	require.Equal(t, 1, keySet2.Len())

	expectedKey2, ok := keySet2.Get(0)
	require.True(t, ok)

	require.Equal(t, expectedKey2, key2)

	// Test that old key doesn't match new key
	require.NotEqual(t, key1, key2)

	// Validate that error is returned when using fake jwks uri
	_, err = newKeyHandler("http://foo.bar/baz", 10*time.Millisecond, 100)
	require.Error(t, err)

	// Validate that error is returned when keys are rotated,
	// new token with new key and jwks uri isn't accessible
	op.RotateKeys(t)
	token3 := op.GetToken(t)
	keyID3, err := getKeyIDFromTokenString(token3.AccessToken)
	require.NoError(t, err)
	op.Close(t)
	_, err = keyHandler.getByKeyID(keyID3)
	require.Error(t, err)
}

func TestUpdate(t *testing.T) {
	op := server.NewTesting(t)
	issuer := op.GetURL(t)
	discoveryUri := getDiscoveryUriFromIssuer(issuer)
	jwksUri, err := getJwksUriFromDiscoveryUri(discoveryUri, 10*time.Millisecond)
	require.NoError(t, err)

	rateLimit := uint(100)
	keyHandler, err := newKeyHandler(jwksUri, 10*time.Millisecond, rateLimit)
	require.NoError(t, err)

	require.Equal(t, 1, keyHandler.keyUpdateCount)

	_, err = keyHandler.waitForUpdateKeySet()
	require.NoError(t, err)

	require.Equal(t, 2, keyHandler.keyUpdateCount)

	concurrentUpdate := func(workers int) {
		wg1 := sync.WaitGroup{}
		wg1.Add(1)

		wg2 := sync.WaitGroup{}
		for i := 0; i < workers; i++ {
			wg2.Add(1)
			go func() {
				wg1.Wait()
				_, err := keyHandler.waitForUpdateKeySet()
				require.NoError(t, err)
				wg2.Done()
			}()
		}
		wg1.Done()
		wg2.Wait()
	}

	concurrentUpdate(100)
	require.Equal(t, 3, keyHandler.keyUpdateCount)
	concurrentUpdate(100)
	require.Equal(t, 4, keyHandler.keyUpdateCount)
	concurrentUpdate(100)
	require.Equal(t, 5, keyHandler.keyUpdateCount)

	multipleConcurrentUpdates := func() {
		wg1 := sync.WaitGroup{}
		wg1.Add(1)

		wg2 := sync.WaitGroup{}
		for i := 0; i < 10; i++ {
			wg2.Add(1)
			go func() {
				wg1.Wait()
				concurrentUpdate(10)
				wg2.Done()
			}()
		}
		wg1.Done()
		wg2.Wait()
	}

	multipleConcurrentUpdates()
	require.Equal(t, 6, keyHandler.keyUpdateCount)

	// test rate limit
	start := time.Now()
	_, err = keyHandler.waitForUpdateKeySet()
	require.NoError(t, err)
	stop := time.Now()
	expectedStop := start.Add(time.Second / time.Duration(rateLimit))

	require.WithinDuration(t, expectedStop, stop, 4*time.Millisecond)

	require.Equal(t, 7, keyHandler.keyUpdateCount)
}
