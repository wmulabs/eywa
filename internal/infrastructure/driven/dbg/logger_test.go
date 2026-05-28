package dbg

import (
	"sync"
	"testing"
)

func TestGetLogger_ReturnsNonNil(t *testing.T) {
	l := GetLogger()
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestCreateLogger_ReturnsNonNil(t *testing.T) {
	l := CreateLogger("test")
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestInitializeLogger_SetsName(t *testing.T) {
	InitializeLogger("my-service")
	l := GetLogger()
	if l == nil {
		t.Fatal("expected non-nil logger after initialization")
	}
}

func TestSetLogger_UsedByGetLogger(t *testing.T) {
	custom := CreateLogger("custom")
	SetLogger(custom)
	l := GetLogger()
	if l != custom {
		t.Fatal("expected GetLogger to return the logger set by SetLogger")
	}
}

func TestSetLogger_ConcurrentWithGetLogger(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			SetLogger(CreateLogger("concurrent"))
		}()
		go func() {
			defer wg.Done()
			l := GetLogger()
			if l == nil {
				t.Error("GetLogger returned nil during concurrent access")
			}
		}()
	}
	wg.Wait()
}
