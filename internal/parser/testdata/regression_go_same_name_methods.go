package p

type A struct{}
type B struct{}

func (a *A) Close() error { return nil }
func (b *B) Close() error { return nil }
