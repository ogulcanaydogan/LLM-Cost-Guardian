package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMainCallsExecute(t *testing.T) {
	called := false
	original := execute
	execute = func() {
		called = true
	}
	t.Cleanup(func() { execute = original })

	main()

	assert.True(t, called)
}
