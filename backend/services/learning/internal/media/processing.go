// Package media provides validation, transcode packaging, and signed playback token helpers.
package media

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	// ErrInvalidPlaybackToken indicates a malformed or tampered playback token.
	ErrInvalidPlaybackToken = errors.New("invalid playback token")
	// ErrExpiredPlaybackToken indicates the playback token has expired.
	ErrExpiredPlaybackToken = errors.New("playback token has expired")
)

// PlaybackTokenClaims represents the signed claims embedded in a short-lived playback token.
type PlaybackTokenClaims struct {
	MediaVersionID string `json:"mv"`
	CourseID       string `json:"cid"`
	LessonID       string `json:"lid"`
	UserID         string `json:"uid"`
	Purpose        string `json:"pur"`
	ExpiresAt      int64  `json:"exp"`
}

// ValidateMedia enforces the MVP video upload baseline rules from the PRD (R-15).
func ValidateMedia(meta MediaMetadata) error {
	if meta.ActualBytes <= 0 {
		return ErrFileEmpty
	}
	if meta.ActualBytes > MaxDeclaredBytes {
		return ErrFileTooLarge
	}

	container := strings.ToLower(strings.TrimSpace(meta.Container))
	if container != "mp4" {
		return ErrInvalidContainer
	}

	codec := strings.ToLower(strings.TrimSpace(meta.Codec))
	if codec != "h264" && codec != "avc1" {
		return ErrInvalidCodec
	}

	if meta.DurationSeconds <= 0 {
		return ErrDurationZero
	}
	if meta.DurationSeconds > MaxDurationSeconds {
		return ErrDurationExceeded
	}

	longSide := meta.Width
	shortSide := meta.Height
	if shortSide > longSide {
		longSide, shortSide = shortSide, longSide
	}

	if longSide <= 0 || shortSide <= 0 || longSide > MaxLongSide || shortSide > MaxShortSide {
		return ErrResolutionExceeded
	}

	if meta.FPS <= 0 || meta.FPS > MaxFPS {
		return ErrFPSExceeded
	}

	return nil
}

// DetermineQualities selects HLS qualities without upsampling.
func DetermineQualities(width, height int) []string {
	longSide := width
	shortSide := height
	if shortSide > longSide {
		longSide, shortSide = shortSide, longSide
	}

	if longSide >= 1920 || shortSide >= 1080 {
		return []string{"720p", "1080p"}
	}
	if longSide >= 1280 || shortSide >= 720 {
		return []string{"720p"}
	}
	return []string{"source"}
}

// SignPlaybackToken creates an HMAC-SHA256 signed token containing claims and expiry.
func SignPlaybackToken(claims PlaybackTokenClaims, secret string) (string, error) {
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
func VerifyPlaybackToken(token, secret string, now time.Time) (*PlaybackTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, ErrInvalidPlaybackToken
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0]))
	expectedSig := mac.Sum(nil)

	providedSig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(providedSig, expectedSig) {
		return nil, ErrInvalidPlaybackToken
	}

	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidPlaybackToken
	}

	var claims PlaybackTokenClaims
	if err := json.Unmarshal(rawPayload, &claims); err != nil {
		return nil, ErrInvalidPlaybackToken
	}
	if claims.ExpiresAt <= now.Unix() {
		return nil, ErrExpiredPlaybackToken
	}
	return &claims, nil
}
