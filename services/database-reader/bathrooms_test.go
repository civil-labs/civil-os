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
