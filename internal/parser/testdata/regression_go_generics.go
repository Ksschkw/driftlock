package p

func Map[T any, U any](xs []T, f func(T) U) []U { return nil }

func (s *Stack[T]) Push(v T) {}

type Stack[T any] struct{ items []T }

type Set[T comparable] map[T]int
