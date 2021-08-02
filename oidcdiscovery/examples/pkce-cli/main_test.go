package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateRandomString(t *testing.T) {
	for i := 0; i < 5; i++ {
		s, err := generateRandomString(32)
		require.NoError(t, err)
		t.Logf("Iteration #%d: %s (%d)", i, s, len(s))
	}
}

func TestGenerateCodeChallengeS256(t *testing.T) {
	for i := 0; i < 5; i++ {
		verifier, challange, err := generateCodeChallengeS256()
		require.NoError(t, err)
		t.Logf("Iteration verifier  #%d: %s (%d)", i, verifier, len(verifier))
		t.Logf("Iteration challange #%d: %s (%d)", i, challange, len(challange))
	}
}

func TestGenerateState(t *testing.T) {
	for i := 0; i < 5; i++ {
		state, err := generateState()
		require.NoError(t, err)
		t.Logf("Iteration #%d: %s (%d)", i, state, len(state))
	}
}
