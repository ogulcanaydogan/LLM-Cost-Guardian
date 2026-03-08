package auth_test

import (
	"net/http/httptest"
	"testing"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKey(t *testing.T) {
	raw, prefix, hash, err := auth.GenerateAPIKey()
	require.NoError(t, err)
	assert.Contains(t, raw, "lcg_")
	assert.Equal(t, raw[:12], prefix)
	assert.Len(t, hash, 64)
	assert.Equal(t, auth.HashAPIKey(raw), hash)
}

func TestExtractAPIKey(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-LCG-API-Key", "lcg_header")
	assert.Equal(t, "lcg_header", auth.ExtractAPIKey(req))

	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer lcg_bearer")
	assert.Equal(t, "lcg_bearer", auth.ExtractAPIKey(req))

	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic abc123")
	assert.Empty(t, auth.ExtractAPIKey(req))
}
