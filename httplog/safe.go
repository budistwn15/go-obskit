package httplog

func SafeExecute(recoverInternally bool, fn func()) (panicked bool) {
	if !recoverInternally {
		fn()
		return false
	}
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	fn()
	return false
}

func SafeValue[T any](recoverInternally bool, fallback T, fn func() T) (value T, panicked bool) {
	if !recoverInternally {
		return fn(), false
	}
	value = fallback
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	value = fn()
	return value, false
}
