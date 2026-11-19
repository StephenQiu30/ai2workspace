// Package media provides validation and transcode packaging rules.
package media

import (
	"strings"
)

// ValidateMedia enforces the MVP video upload baseline rules from the PRD.
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

	if longSide > MaxLongSide || shortSide > MaxShortSide {
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
