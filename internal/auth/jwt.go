package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrTokenInvalid = errors.New("недействительный токен")

const (
	AccessTTL  = 15 * time.Minute   // ТЗ: access 15 минут
	RefreshTTL = 7 * 24 * time.Hour // ТЗ: refresh 7 дней
)

type Claims struct {
	jwt.RegisteredClaims
	Group     string `json:"grp,omitempty"`
	TokenType string `json:"typ"`
}

// TokenManager выпускает пары токенов; refresh-токены одноразовые:
// jti хранится в Redis и сжигается при обновлении (ротация).
type TokenManager struct {
	secret []byte
	rdb    *redis.Client
}

func NewTokenManager(secret string, rdb *redis.Client) *TokenManager {
	return &TokenManager{secret: []byte(secret), rdb: rdb}
}

func (t *TokenManager) IssuePair(ctx context.Context, pid uuid.UUID, group string) (access, refresh string, err error) {
	now := time.Now()

	access, err = jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   pid.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)),
		},
		Group:     group,
		TokenType: "access",
	}).SignedString(t.secret)
	if err != nil {
		return "", "", err
	}

	jti := uuid.NewString()
	refresh, err = jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   pid.String(),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(RefreshTTL)),
		},
		Group:     group,
		TokenType: "refresh",
	}).SignedString(t.secret)
	if err != nil {
		return "", "", err
	}

	if err := t.rdb.Set(ctx, "refresh:"+jti, pid.String(), RefreshTTL).Err(); err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func (t *TokenManager) parse(token, wantType string) (*Claims, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(token, &claims, func(tk *jwt.Token) (any, error) {
		if _, ok := tk.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return t.secret, nil
	})
	if err != nil || claims.TokenType != wantType {
		return nil, ErrTokenInvalid
	}
	return &claims, nil
}

// ParseAccess проверяет access-токен и возвращает (participant_id, group).
func (t *TokenManager) ParseAccess(token string) (uuid.UUID, string, error) {
	claims, err := t.parse(token, "access")
	if err != nil {
		return uuid.Nil, "", err
	}
	pid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, "", ErrTokenInvalid
	}
	return pid, claims.Group, nil
}

// RotateRefresh сжигает старый refresh (одноразовость) и возвращает субъекта.
func (t *TokenManager) RotateRefresh(ctx context.Context, token string) (uuid.UUID, string, error) {
	claims, err := t.parse(token, "refresh")
	if err != nil {
		return uuid.Nil, "", err
	}
	stored, err := t.rdb.GetDel(ctx, "refresh:"+claims.ID).Result()
	if errors.Is(err, redis.Nil) || (err == nil && stored != claims.Subject) {
		// уже использован или подделан — вся ветка токенов недействительна
		return uuid.Nil, "", ErrTokenInvalid
	}
	if err != nil {
		return uuid.Nil, "", err
	}
	pid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, "", ErrTokenInvalid
	}
	return pid, claims.Group, nil
}

// RevokeRefresh — выход: сжигаем refresh, если он валиден.
func (t *TokenManager) RevokeRefresh(ctx context.Context, token string) {
	if claims, err := t.parse(token, "refresh"); err == nil {
		t.rdb.Del(ctx, "refresh:"+claims.ID)
	}
}
