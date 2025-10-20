package obs

import "testing"

func TestInitLogger(t *testing.T) {
	InitLogger()
	if Logger == nil {
		t.Fatalf("logger is nil")
	}
}
