package main

import "testing"

func TestBathroomTotal(t *testing.T) {
	cases := []struct {
		full, half int32
		want       float64
	}{
		{0, 0, 0},
		{2, 0, 2},
		{2, 1, 2.5},
		{3, 2, 4},
		{0, 1, 0.5},
	}
	for _, c := range cases {
		if got := bathroomTotal(c.full, c.half); got != c.want {
			t.Errorf("bathroomTotal(%d, %d) = %v, want %v", c.full, c.half, got, c.want)
		}
	}
}

func TestImprovementBathrooms(t *testing.T) {
	n := func(v int32) *int32 { return &v }
	cases := []struct {
		full, half *int32
		want       *float64
	}{
		{nil, nil, nil},
		{n(2), nil, ptr(2.0)},
		{nil, n(1), ptr(0.5)},
		{n(2), n(1), ptr(2.5)},
		{n(0), nil, ptr(0.0)},
	}
	for _, c := range cases {
		got := improvementBathrooms(c.full, c.half)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("improvementBathrooms(%v, %v) = %v, want %v", c.full, c.half, got, c.want)
		}
	}
}

func ptr[T any](v T) *T { return &v }
