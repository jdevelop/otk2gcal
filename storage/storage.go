package storage

import "msclnd/auth"

type Storage interface {
	LoadTokens() (*auth.Tokens, error)
	SaveTokens(*auth.Tokens) error
}
