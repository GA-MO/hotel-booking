package upload

import "errors"

var (
	// ErrUnsupportedKind covers both invalid `kind` values and disallowed
	// content types — both surface as 400 BAD_REQUEST.
	ErrUnsupportedKind    = errors.New("unsupported upload kind or content type")
	ErrSizeExceeded       = errors.New("upload size exceeds limit")
	ErrInvalidContentType = errors.New("invalid or disallowed content type")
	ErrInvalidRequest     = errors.New("invalid request")
	ErrNotConfigured      = errors.New("storage signer not configured")
)
