package oerror

import "fmt"

type KillLimeError struct {
	Err string
}

func New(err string, a ...any) *KillLimeError {
	return &KillLimeError{Err: fmt.Sprintf(err, a...)}
}

func (e *KillLimeError) Error() string {
	return e.Err
}
