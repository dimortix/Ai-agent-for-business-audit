package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"+79001234567":     "+79001234567",
		"79001234567":      "+79001234567",
		"89001234567":      "+79001234567",
		"8 900 123-45-67":  "+79001234567",
		"+7 (900) 1234567": "+79001234567",
		"9001234567":       "+79001234567",
	}
	for in, want := range cases {
		got, err := NormalizePhone(in)
		assert.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	for _, bad := range []string{"", "123", "абв", "12345"} {
		_, err := NormalizePhone(bad)
		assert.Error(t, err, bad)
	}
}

func TestParseTxType(t *testing.T) {
	for _, in := range []string{"income", "ПРИХОД", " sale "} {
		got, err := parseTxType(in)
		assert.NoError(t, err)
		assert.Equal(t, "income", got)
	}
	for _, in := range []string{"return", "возврат", "Refund"} {
		got, err := parseTxType(in)
		assert.NoError(t, err)
		assert.Equal(t, "return", got)
	}
	_, err := parseTxType("перевод")
	assert.Error(t, err)
}

func TestParseDate(t *testing.T) {
	d1, err := parseDate("2026-07-14")
	assert.NoError(t, err)
	d2, err2 := parseDate("14.07.2026")
	assert.NoError(t, err2)
	assert.True(t, d1.Equal(d2))

	_, err = parseDate("07/14/2026")
	assert.Error(t, err)
}
