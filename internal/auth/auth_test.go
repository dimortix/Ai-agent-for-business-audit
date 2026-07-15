package auth

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestOTPGenerateVerify(t *testing.T) {
	ctx := context.Background()
	otp := NewOTPManager(testRedis(t))

	code, err := otp.Generate(ctx, "+79001234567")
	require.NoError(t, err)
	require.Len(t, code, 4)

	assert.ErrorIs(t, otp.Verify(ctx, "+79001234567", "0000"), ErrCodeInvalid)
	assert.NoError(t, otp.Verify(ctx, "+79001234567", code))
	// код одноразовый
	assert.ErrorIs(t, otp.Verify(ctx, "+79001234567", code), ErrCodeInvalid)
}

func TestOTPRequestRateLimit(t *testing.T) {
	ctx := context.Background()
	otp := NewOTPManager(testRedis(t))

	for i := 0; i < 5; i++ {
		_, err := otp.Generate(ctx, "+79990000000")
		require.NoError(t, err, "запрос %d из 5 должен проходить", i+1)
	}
	_, err := otp.Generate(ctx, "+79990000000")
	assert.ErrorIs(t, err, ErrRateLimited, "шестой запрос за час — отказ")
}

func TestOTPAttemptLimit(t *testing.T) {
	ctx := context.Background()
	otp := NewOTPManager(testRedis(t))

	code, err := otp.Generate(ctx, "+79991111111")
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		assert.ErrorIs(t, otp.Verify(ctx, "+79991111111", "9999"), ErrCodeInvalid)
	}
	// шестая попытка сжигает код
	assert.ErrorIs(t, otp.Verify(ctx, "+79991111111", "9999"), ErrTooManyAttempts)
	assert.ErrorIs(t, otp.Verify(ctx, "+79991111111", code), ErrTooManyAttempts,
		"после перебора даже верный код не принимается")
}

func TestTokensIssueParseRotate(t *testing.T) {
	ctx := context.Background()
	tm := NewTokenManager("test-secret", testRedis(t))
	pid := uuid.New()

	access, refresh, err := tm.IssuePair(ctx, pid, "B")
	require.NoError(t, err)

	gotPID, group, err := tm.ParseAccess(access)
	require.NoError(t, err)
	assert.Equal(t, pid, gotPID)
	assert.Equal(t, "B", group)

	// refresh не подходит как access и наоборот
	_, _, err = tm.ParseAccess(refresh)
	assert.ErrorIs(t, err, ErrTokenInvalid)

	// ротация: старый refresh сжигается
	rotPID, rotGroup, err := tm.RotateRefresh(ctx, refresh)
	require.NoError(t, err)
	assert.Equal(t, pid, rotPID)
	assert.Equal(t, "B", rotGroup)

	_, _, err = tm.RotateRefresh(ctx, refresh)
	assert.ErrorIs(t, err, ErrTokenInvalid, "повторное использование refresh — отказ")
}

func TestTokensWrongSecret(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	tm1 := NewTokenManager("secret-one", rdb)
	tm2 := NewTokenManager("secret-two", rdb)

	access, _, err := tm1.IssuePair(ctx, uuid.New(), "B")
	require.NoError(t, err)

	_, _, err = tm2.ParseAccess(access)
	assert.ErrorIs(t, err, ErrTokenInvalid)
}
