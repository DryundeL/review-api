package contract

import "errors"

var (
	ErrInvalid = errors.New("invalid image")
	// ErrUnsupportedFormat: the content is not JPEG, PNG or WebP.
	ErrUnsupportedFormat = errors.New("unsupported image format")
	// ErrTooLarge: the file exceeds MaxImageBytes.
	ErrTooLarge = errors.New("image too large")
	// ErrDimensions: width or height is outside 1..MaxImageSide.
	ErrDimensions = errors.New("image dimensions out of range")
	// ErrQuotaExceeded: the uploader hit the hourly upload quota.
	ErrQuotaExceeded = errors.New("upload quota exceeded")
)

const (
	// MaxImageBytes is the upload limit from SPEC-lists (300 KB).
	MaxImageBytes = 300 * 1024
	// MaxImageSide bounds decoded size so a small file cannot expand into a huge bitmap on clients.
	MaxImageSide = 4096
)
