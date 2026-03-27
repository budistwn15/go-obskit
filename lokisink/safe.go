package lokisink

func safeCall(recoverInternally bool, fn func()) {
	if fn == nil {
		return
	}
	if !recoverInternally {
		fn()
		return
	}
	defer func() { _ = recover() }()
	fn()
}
