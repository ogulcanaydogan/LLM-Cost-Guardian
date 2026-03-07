package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfigPath(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	pricingDir := filepath.Clean(filepath.Join(wd, "..", "..", "pricing"))

	configData := []byte("storage:\n" +
		"  path: " + filepath.Join(dir, "guardian.db") + "\n" +
		"proxy:\n" +
		`  listen: "127.0.0.1:0"` + "\n" +
		"  read_timeout: 30s\n" +
		"  write_timeout: 60s\n" +
		"  max_body_size: 1024\n" +
		"pricing:\n" +
		"  dir: " + pricingDir + "\n" +
		"logging:\n" +
		"  level: error\n" +
		"  format: text\n" +
		"defaults:\n" +
		"  project: default\n")
	require.NoError(t, os.WriteFile(cfgPath, configData, 0o644))
	return cfgPath
}

func TestMainCallsRunMain(t *testing.T) {
	called := false
	original := runMain
	runMain = func() error {
		called = true
		return nil
	}
	t.Cleanup(func() { runMain = original })

	main()

	assert.True(t, called)
}

func TestRun_ShutsDownWithContext(t *testing.T) {
	cfgPath := testConfigPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	require.NoError(t, run(ctx, cfgPath))
}
