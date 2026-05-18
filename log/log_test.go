package log

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/sagernet/sing-box/option"
)

func TestLogFormatJSONAccepted(t *testing.T) {
	var buf bytes.Buffer
	factory, err := New(Options{Context: context.Background(), Options: option.LogOptions{Format: "json"}, DefaultWriter: &buf, BaseTime: time.Unix(0, 0)})
	if err != nil {
		t.Fatal(err)
	}
	factory.Logger().Info("hello")
	if !bytes.Contains(buf.Bytes(), []byte(`"level":"info"`)) {
		t.Fatalf("expected json log, got %q", buf.String())
	}
}

func TestLogFormatInvalidRejected(t *testing.T) {
	_, err := New(Options{Context: context.Background(), Options: option.LogOptions{Format: "xml"}})
	if err == nil {
		t.Fatal("expected invalid format error")
	}
}

func TestLogFormatEmptyPreservesText(t *testing.T) {
	var buf bytes.Buffer
	factory, err := New(Options{Context: context.Background(), Options: option.LogOptions{}, DefaultWriter: &buf, BaseTime: time.Unix(0, 0)})
	if err != nil {
		t.Fatal(err)
	}
	factory.Logger().Info("hello")
	if bytes.Contains(buf.Bytes(), []byte(`"level":"info"`)) || !bytes.Contains(buf.Bytes(), []byte("INFO")) {
		t.Fatalf("expected text log, got %q", buf.String())
	}
}
