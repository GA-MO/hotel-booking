package notification

import "errors"

var (
	ErrNotificationNotFound = errors.New("notification not found")
	ErrInvalidChannel       = errors.New("invalid channel")
	ErrInvalidTemplate      = errors.New("invalid template")
	ErrInvalidRecipient     = errors.New("invalid recipient")
	ErrTemplateRender       = errors.New("template render failed")
	ErrSenderUnavailable    = errors.New("sender unavailable")
)
