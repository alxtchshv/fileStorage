package service

import (
	"io"

	"managerFiles/pkg/encrypt"
)

type encryptionService struct {
	cipher *encrypt.AES
}

func NewEncryptionService(cipher *encrypt.AES) EncryptionService {
	return &encryptionService{cipher: cipher}
}

func (s *encryptionService) Encrypt(dst io.Writer, src io.Reader) error {
	return s.cipher.EncryptStream(dst, src)
}

func (s *encryptionService) Decrypt(dst io.Writer, src io.Reader) error {
	return s.cipher.DecryptStream(dst, src)
}
