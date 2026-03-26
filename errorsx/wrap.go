package errorsx

import "errors"

func Wrap(err error, meta Meta) error {
	if err == nil {
		return nil
	}
	return &Error{
		err:  err,
		Meta: meta,
	}
}

func Extract(err error) (*Error, bool) {
	if err == nil {
		return nil, false
	}
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
