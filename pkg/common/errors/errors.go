package errors

import (
	"errors"
	"fmt"
)

func Wrap(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}

func New(msg string) error {
	return errors.New(msg)
}
