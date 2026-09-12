package sample

type Stack[T any] struct{ items []T }

type Set[T comparable] map[T]struct{}

func Map[T any, U any](xs []T, f func(T) U) []U { return nil }

func Apply(xs []int, f func(int) int) []int { return nil }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

func (s *Stack[T]) Pop() (T, bool) { return s.items[0], true }

func Long(
	a int,
	b string,
) (string, error) {
	return "", nil
}
