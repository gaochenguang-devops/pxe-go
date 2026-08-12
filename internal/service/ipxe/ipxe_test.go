package ipxe

import (
	"strings"
	"testing"
)

func TestGetSerialConsole(t *testing.T) {
	if !strings.Contains(getSerialConsole("aarch64"), "ttyAMA0") {
		t.Error("arm serial console wrong")
	}
	if !strings.Contains(getSerialConsole("x86_64"), "ttyS0") {
		t.Error("x86 serial console wrong")
	}
}
