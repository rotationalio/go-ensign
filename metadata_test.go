package ensign_test

import (
	"testing"

	"github.com/rotationalio/go-ensign"
	"github.com/stretchr/testify/require"
)

func TestMetadata(t *testing.T) {
	meta := make(ensign.Metadata)
	require.Empty(t, meta.Get("key"), "expected empty string when key doesn't exist")

	meta.Set("key", "value")
	require.Equal(t, "value", meta.Get("key"), "should be able to get and set key/value pair")
}
