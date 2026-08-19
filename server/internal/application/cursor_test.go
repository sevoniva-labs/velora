package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCursorRoundTrip(t *testing.T) {
	enc := encodeListCursor(true, 5, "devops", 42)
	assert.NotEmpty(t, enc)

	dec, err := decodeListCursor(enc)
	require.NoError(t, err)
	require.NotNil(t, dec)
	assert.Equal(t, true, dec.Featured)
	assert.Equal(t, 5, dec.Sort)
	assert.Equal(t, "devops", dec.NameLower)
	assert.Equal(t, uint64(42), dec.ID)
}

func TestDecodeListCursorEmpty(t *testing.T) {
	dec, err := decodeListCursor("")
	require.NoError(t, err)
	assert.Nil(t, dec, "空游标应返回 nil（走 OFFSET 分页）")
}

func TestDecodeListCursorInvalid(t *testing.T) {
	_, err := decodeListCursor("not-base64!!")
	assert.Error(t, err)
}
