package outbound

func safeDo(recoverInternally bool, fn func()) {
	if !recoverInternally {
		fn()
		return
	}
	defer func() {
		_ = recover()
	}()
	fn()
}

func safeLog(recoverInternally bool, fn func()) {
	safeDo(recoverInternally, fn)
}
