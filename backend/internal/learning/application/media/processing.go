package media

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	mediadomain "github.com/StephenQiu30/ai2workspace/backend/internal/learning/domain/media"
)

// SignPlaybackToken creates an HMAC-SHA256 signed token containing claims and expiry.
func SignPlaybackToken(claims mediadomain.PlaybackTokenClaims, secret string) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(encodedPayload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encodedPayload + "." + sig, nil
}

// VerifyPlaybackToken validates signature and expiry of a signed playback token.
func VerifyPlaybackToken(token, secret string, now time.Time) (*mediadomain.PlaybackTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, mediadomain.ErrInvalidPlaybackToken
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0]))
	expectedSig := mac.Sum(nil)

	providedSig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(providedSig, expectedSig) {
		return nil, mediadomain.ErrInvalidPlaybackToken
	}

	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, mediadomain.ErrInvalidPlaybackToken
	}

	var claims mediadomain.PlaybackTokenClaims
	if err := json.Unmarshal(rawPayload, &claims); err != nil {
		return nil, mediadomain.ErrInvalidPlaybackToken
	}
	if claims.ExpiresAt <= now.Unix() {
		return nil, mediadomain.ErrExpiredPlaybackToken
	}
	return &claims, nil
}
