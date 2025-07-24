package mmcache

import (
	"testing"
)

func CmpError(t *testing.T, err error) {
	if err == nil {
		t.Fatalf("CmpError: %+v", err)
	}
}

func CmpNoError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("CmpNoError: %+v", err)
	}
}

func Cmp[ValueT comparable](t *testing.T, got, expected ValueT) {
	if got != expected {
		t.Fatalf("Cmp: got=%+v, expected=%+v", got, expected)
	}
}
