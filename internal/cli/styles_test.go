package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestSetOutputRedirects proves the output seam works: a Print* helper writes
// to the buffers supplied via SetOutput, and the restore function reinstates
// the previous writers.
func TestSetOutputRedirects(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var out, errOut bytes.Buffer
	restore := SetOutput(&out, &errOut)
	defer restore()

	PrintInfo("hello world")
	PrintError("boom")

	if got := out.String(); !strings.Contains(got, "hello world") {
		t.Errorf("stdout = %q; want it to contain %q", got, "hello world")
	}
	if got := errOut.String(); !strings.Contains(got, "boom") {
		t.Errorf("stderr = %q; want it to contain %q", got, "boom")
	}

	restore()
	if stdout == nil || stderr == nil {
		t.Fatal("restore left package writers nil")
	}
}
