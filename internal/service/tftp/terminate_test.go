package tftp

import (
	"errors"
	"testing"
)

func TestIsClientTerminate(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("sending block 0: code=8, error: User aborted the transfer"), true},
		{errors.New("sending block 5: code=8, error: Terminate transfer"), true},
		{errors.New("sending block 3: code=0, error: file not found"), false},
		{errors.New("network timeout"), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := isClientTerminate(c.err); got != c.want {
			t.Errorf("isClientTerminate(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
