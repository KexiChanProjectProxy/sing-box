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

func TestLogFormatEmptyDefaultsJSON(t *testing.T) {
	var buf bytes.Buffer
	factory, err := New(Options{Context: context.Background(), Options: option.LogOptions{}, DefaultWriter: &buf, BaseTime: time.Unix(0, 0)})
	if err != nil {
		t.Fatal(err)
	}
	factory.Logger().Info("hello")
	if !bytes.Contains(buf.Bytes(), []byte(`"level":"info"`)) {
		t.Fatalf("expected json log, got %q", buf.String())
	}
}

func TestLogFormatTextRejected(t *testing.T) {
	_, err := New(Options{Context: context.Background(), Options: option.LogOptions{Format: "text"}})
	if err == nil {
		t.Fatal("expected text format error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("text has been removed")) {
		t.Fatalf("expected text-removed error, got %v", err)
	}
}
