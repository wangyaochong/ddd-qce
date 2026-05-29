package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVersion_LdflagsInjected(t *testing.T) {
	original := Version
	Version = "v20260529.v1"
	defer func() { Version = original }()

	assert.Equal(t, "v20260529.v1", GetVersion())
}

func TestGetVersion_DevelFallback(t *testing.T) {
	original := Version
	Version = "(devel)"
	defer func() { Version = original }()

	v := GetVersion()
	assert.NotEqual(t, "v20260529.v1", v)
	if strings.HasPrefix(v, "v") {
		assert.True(t, strings.Contains(v, "-") || strings.Contains(v, "."), "devel version should contain commit or date info")
	}
}

func TestGetVersion_DevelFormat(t *testing.T) {
	original := Version
	Version = "(devel)"
	defer func() { Version = original }()

	v := GetVersion()
	if v != "(devel)" {
		assert.True(t,
			strings.HasPrefix(v, "v2") || len(v) >= 7,
			"version should start with date prefix or be a commit hash: %s", v,
		)
	}
}
