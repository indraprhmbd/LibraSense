package store

import (
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	database, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("buka store: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.EnsureDevice("dev-1", "token-perangkat-rahasia", "uji"); err != nil {
		t.Fatalf("daftar perangkat: %v", err)
	}

	return database
}

func value(number float64) *float64 {
	return &number
}

func TestReadingExistsMatchesDeviceAndTimestamp(t *testing.T) {
	database := newTestStore(t)

	sensors := []Sensor{{ID: "dht1", Type: "temperature", Value: value(28), OK: true, Rack: intPtr(1)}}
	if err := database.SaveReading("dev-1", 1763520000000, sensors, "192.168.0.10", "WiFi", "test", nil); err != nil {
		t.Fatalf("simpan: %v", err)
	}

	found, err := database.ReadingExists("dev-1", 1763520000000)
	if err != nil {
		t.Fatalf("cek: %v", err)
	}
	if !found {
		t.Error("timestamp yang tersimpan harus ditemukan")
	}

	other, err := database.ReadingExists("dev-1", 1763520005000)
	if err != nil {
		t.Fatalf("cek: %v", err)
	}
	if other {
		t.Error("timestamp lain tidak boleh dianggap sudah ada")
	}
}

func TestSaveReadingPrunesOldRows(t *testing.T) {
	database := newTestStore(t)
	database.readingKeep = 50

	sensors := []Sensor{{ID: "dht1", Type: "temperature", Value: value(28), OK: true, Rack: intPtr(1)}}

	for index := 0; index < 60; index++ {
		if err := database.SaveReading("dev-1", int64(1000+index), sensors, "", "", "", nil); err != nil {
			t.Fatalf("simpan %d: %v", index, err)
		}
	}

	history, err := database.History("dev-1", 100)
	if err != nil {
		t.Fatalf("riwayat: %v", err)
	}
	if len(history) != 50 {
		t.Fatalf("baris tersimpan = %d, mau 50", len(history))
	}
	if history[0].TS != 1010 {
		t.Errorf("baris tertua ts = %d, mau 1010", history[0].TS)
	}
	if history[len(history)-1].TS != 1059 {
		t.Errorf("baris terbaru ts = %d, mau 1059", history[len(history)-1].TS)
	}
}

func intPtr(number int) *int {
	return &number
}
