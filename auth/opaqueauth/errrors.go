package opaqueauth

import "errors"

var (
	ErrInvalidToken = errors.New("opaque: invalid or non-existent token")
	ErrExpiredToken = errors.New("opaque: token has expired")
)