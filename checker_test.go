package main

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		statusCode int
		want       checkStatus
	}{
		{199, statusFailure},
		{200, statusHealthy},
		{299, statusHealthy},
		{300, statusFailure},
		{301, statusFailure},
		{401, statusReachable},
		{403, statusReachable},
		{404, statusFailure},
		{500, statusFailure},
	}

	for _, c := range cases {
		got := classify(c.statusCode)
		if got != c.want {
			t.Errorf("classify(%d) = %v, want %v \n", c.statusCode, got, c.want)
		}
	}
}
