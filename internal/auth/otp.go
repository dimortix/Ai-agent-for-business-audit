// Package auth — OTP-коды по телефону и JWT-сессии.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrRateLimited     = errors.New("слишком много запросов кода, попробуйте через час")
	ErrCodeInvalid     = errors.New("неверный или просроченный код")
	ErrTooManyAttempts = errors.New("слишком много попыток, запросите новый код")
)

const (
	otpTTL         = 5 * time.Minute // ТЗ: OTP живёт 5 минут
	maxRequests    = 5               // запросов кода на номер в час
	maxAttempts    = 5               // попыток ввода на один код
	requestsWindow = time.Hour
)

type OTPManager struct {
	rdb *redis.Client
}

func NewOTPManager(rdb *redis.Client) *OTPManager {
	return &OTPManager{rdb: rdb}
}

// Generate создаёт 4-значный код и кладёт его в Redis (otp:<phone>, TTL 5 мин).
func (o *OTPManager) Generate(ctx context.Context, phone string) (string, error) {
	reqs, err := o.rdb.Incr(ctx, "otp_req:"+phone).Result()
	if err != nil {
		return "", err
	}
	if reqs == 1 {
		o.rdb.Expire(ctx, "otp_req:"+phone, requestsWindow)
	}
	if reqs > maxRequests {
		return "", ErrRateLimited
	}

	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("%04d", n.Int64())

	pipe := o.rdb.TxPipeline()
	pipe.Set(ctx, "otp:"+phone, code, otpTTL)
	pipe.Del(ctx, "otp_try:"+phone)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", err
	}
	return code, nil
}

// Verify сверяет код; после 5 неверных попыток код сжигается.
func (o *OTPManager) Verify(ctx context.Context, phone, code string) error {
	tries, err := o.rdb.Incr(ctx, "otp_try:"+phone).Result()
	if err != nil {
		return err
	}
	if tries == 1 {
		o.rdb.Expire(ctx, "otp_try:"+phone, otpTTL)
	}
	if tries > maxAttempts {
		o.rdb.Del(ctx, "otp:"+phone)
		return ErrTooManyAttempts
	}

	stored, err := o.rdb.Get(ctx, "otp:"+phone).Result()
	if errors.Is(err, redis.Nil) {
		return ErrCodeInvalid
	}
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(stored), []byte(code)) != 1 {
		return ErrCodeInvalid
	}

	o.rdb.Del(ctx, "otp:"+phone, "otp_try:"+phone)
	return nil
}
