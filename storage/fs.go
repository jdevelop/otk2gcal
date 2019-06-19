package storage

import (
	"encoding/json"
	"io/ioutil"
	"msclnd/auth"
	"os"
)

type FileStorage struct {
	filePath string
}

func NewFileStorage(path string) *FileStorage {
	return &FileStorage{filePath: path}
}

func (fs *FileStorage) LoadTokens() (*auth.Tokens, error) {
	r, err := os.Open(fs.filePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var t auth.Tokens
	if err := json.NewDecoder(r).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (fs *FileStorage) SaveTokens(tokens *auth.Tokens) error {
	data, err := json.Marshal(tokens)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fs.filePath, data, 0600)
}

var _ Storage = &FileStorage{}
