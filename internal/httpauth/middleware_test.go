package httpauth_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ogulcanaydogan/LLM-Cost-Guardian/internal/httpauth"
	keyauth "github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/auth"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAuthStore(t *testing.T) storage.Storage {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "auth.db")
	store, err := storage.NewSQLite(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestMiddleware_DisabledUsesDefaultTenant(t *testing.T) {
	store := setupAuthStore(t)
	middleware := httpauth.New(store, false, "default", "", testLogger())

	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := httpauth.IdentityFromContext(r.Context())
		require.True(t, ok)
		assert.True(t, identity.Admin)
		_, _ = w.Write([]byte(identity.Tenant.Slug))
	}))

	req := httptest.NewRequest("GET", "/api/v1/summary", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "default", w.Body.String())
}

func TestMiddleware_AuthenticatesAPIKey(t *testing.T) {
	store := setupAuthStore(t)
	ctx := context.Background()
	tenant, err := store.EnsureTenant(ctx, "acme", "Acme")
	require.NoError(t, err)

	rawKey, prefix, hash, err := keyauth.GenerateAPIKey()
	require.NoError(t, err)
	require.NoError(t, store.CreateAPIKey(ctx, &model.APIKey{
		TenantID:  tenant.ID,
		Name:      "primary",
		KeyPrefix: prefix,
		KeyHash:   hash,
		Status:    model.APIKeyStatusActive,
	}))

	middleware := httpauth.New(store, true, "default", "", testLogger())
	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := httpauth.IdentityFromContext(r.Context())
		require.True(t, ok)
		_, _ = w.Write([]byte(identity.Tenant.Slug))
	}))

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("X-LCG-API-Key", rawKey)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "acme", w.Body.String())
}

func TestMiddleware_RejectsInvalidKey(t *testing.T) {
	store := setupAuthStore(t)
	middleware := httpauth.New(store, true, "default", "", testLogger())

	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/v1/usage", nil)
	req.Header.Set("X-LCG-API-Key", "lcg_invalid")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMiddleware_BootstrapAdminKey(t *testing.T) {
	store := setupAuthStore(t)
	middleware := httpauth.New(store, true, "default", "bootstrap-secret", testLogger())

	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := httpauth.IdentityFromContext(r.Context())
		require.True(t, ok)
		assert.True(t, identity.Admin)
		_, _ = w.Write([]byte(identity.Tenant.Slug))
	}))

	req := httptest.NewRequest("GET", "/api/v1/summary", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-secret")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "default", w.Body.String())
}
