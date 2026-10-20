package pagination

import (
	"math"
	"testing"
)

func TestBounds(t *testing.T) {
	if n := (Query{Page: math.MaxInt, Limit: 100}).Offset(); n != math.MaxInt {
		t.Fatal(n)
	}
	if n := (Query{}).Offset(); n != 0 {
		t.Fatal(n)
	}
	for _, tc := range []struct{ total, limit, want int }{{0, 10, 0}, {11, 10, 2}, {-1, 10, 0}, {math.MaxInt, 1, math.MaxInt}} {
		v := NewResult[int](nil, tc.total, Query{Limit: tc.limit})
		if v.Meta.TotalPages != tc.want || v.Items == nil {
			t.Fatal(v)
		}
	}
}
