package kafka

import "errors"

var (
	ErrNonRetryable = errors.New("non-retryable consumer error")
	ErrRetryable    = errors.New("retryable consumer error")
)

type classifiedError struct {
	kind error
	err  error
}

func NonRetryable(err error) error {
	return classify(err, ErrNonRetryable)
}

func Retryable(err error) error {
	return classify(err, ErrRetryable)
}

func classify(err, kind error) error {
	if err == nil {
		return nil
	}
	return classifiedError{kind: kind, err: err}
}

func (e classifiedError) Error() string {
	return e.err.Error()
}

func (e classifiedError) Unwrap() error {
	return e.err
}

func (e classifiedError) Is(target error) bool {
	return target == e.kind
}
