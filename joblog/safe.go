package joblog

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
