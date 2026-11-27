package account

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const computerSecretEntry = "remote-host"

func (s *Service) GetHostSecret(ref string) (string, bool, error) {
	value, err := s.credentials.read(computerSecretEntry, ref)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", false, nil
	}
	return value, err == nil, err
}

func (s *Service) PutHostSecret(ref, value string) error {
	return s.credentials.write(computerSecretEntry, ref, value)
}

func (s *Service) DeleteHostSecret(ref string) error {
	return s.credentials.remove(computerSecretEntry, ref)
}
