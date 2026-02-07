package cache

import (
	"testing"
	"time"
)

func TestStoreGetSet(t *testing.T) {
	store := NewStore()
	store.Set("key", "value", time.Minute)
	val, ok := store.Get("key")
	if !ok {
		t.Fatalf("expected key to be present")
	}
	if val.(string) != "value" {
		t.Fatalf("unexpected value: %v", val)
	}
}

func TestStoreExpiry(t *testing.T) {
	store := NewStore()
	store.Set("key", "value", 5*time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	if _, ok := store.Get("key"); ok {
		t.Fatalf("expected key to expire")
	}
}

func TestStoreDelete(t *testing.T) {
	store := NewStore()
	store.Set("key", "value", time.Minute)
	store.Delete("key")
	if _, ok := store.Get("key"); ok {
		t.Fatalf("expected key to be deleted")
	}
}
