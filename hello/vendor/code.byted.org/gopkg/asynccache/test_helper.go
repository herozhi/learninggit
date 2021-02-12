package asynccache

import "reflect"

// testingTB is a subset of common methods between *testing.T and *testing.B.
type testingTB interface {
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Helper()
}

// Assert .
func Assert(t testingTB, cond bool) {
	t.Helper()
	if !cond {
		t.Fatal("assertion failed")
	}
}

// Assertf .
func Assertf(t testingTB, cond bool, format string, val ...interface{}) {
	t.Helper()
	if !cond {
		t.Fatalf(format, val...)
	}
}

// DeepEqual .
func DeepEqual(t testingTB, a, b interface{}) {
	t.Helper()
	if !reflect.DeepEqual(a, b) {
		t.Fatal("assertion failed")
	}
}
