package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"librasense/internal/config"
	"librasense/internal/store"
)

const testToken = "token-perangkat-rahasia"

type testEnv struct {
	server *Server
	store  *store.Store
	cfg    *config.Config
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()

	cfg := &config.Config{
		Env:          "test",
		Port:         8787,
		DataDir:      filepath.Join(t.TempDir(), "data"),
		DeviceID:     "perpustakaan-1",
		DeviceToken:  testToken,
		BypassAccess: true,
	}

	database, err := store.Open(cfg.DataDir)
	if err != nil {
		t.Fatalf("buka store: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.EnsureDevice(cfg.DeviceID, cfg.DeviceToken, "Perpustakaan Pintar"); err != nil {
		t.Fatalf("daftar perangkat: %v", err)
	}

	return &testEnv{server: New(cfg, database, nil), store: database, cfg: cfg}
}

func (e *testEnv) request(t *testing.T, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		payload = encoded
	}

	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	e.server.Handler().ServeHTTP(recorder, request)
	return recorder
}

func deviceHeaders() map[string]string {
	return map[string]string{"Authorization": "Bearer " + testToken}
}

func reading(temperature float64, extraSensors ...map[string]any) map[string]any {
	sensors := []any{
		map[string]any{"id": "dht1", "type": "temperature", "value": temperature, "ok": true, "unit": "C", "rack": 1},
		map[string]any{"id": "sound1", "type": "noise", "value": 300, "ok": true, "unit": "adc", "rack": 1},
	}
	for _, sensor := range extraSensors {
		sensors = append(sensors, sensor)
	}

	return map[string]any{
		"firmware": "test-1",
		"ip":       "192.168.18.30",
		"mode":     "WiFi",
		"sensors":  sensors,
	}
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", recorder.Body.String(), err)
	}
	return body
}

func TestRejectsMissingDeviceToken(t *testing.T) {
	env := newEnv(t)
	response := env.request(t, http.MethodPost, "/device/readings", reading(27), nil)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, mau 401", response.Code)
	}
}

func TestRejectsWrongDeviceToken(t *testing.T) {
	env := newEnv(t)
	response := env.request(t, http.MethodPost, "/device/readings", reading(27), map[string]string{"Authorization": "Bearer salah"})

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, mau 401", response.Code)
	}
}

func TestIngestStoresReadingAndDecidesActuators(t *testing.T) {
	env := newEnv(t)
	response := env.request(t, http.MethodPost, "/device/readings", reading(35), deviceHeaders())

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", response.Code, response.Body.String())
	}

	body := decode(t, response)
	state := body["state"].(map[string]any)

	if state["fan"] != true {
		t.Errorf("fan = %v, mau true karena suhu di atas ambang", state["fan"])
	}

	buzzer := state["buzzer"].([]any)
	if buzzer[0] != true {
		t.Errorf("buzzer rak 1 = %v, mau true", buzzer[0])
	}

	overview := env.request(t, http.MethodGet, "/api/overview", nil, nil)
	if overview.Code != http.StatusOK {
		t.Fatalf("overview status = %d", overview.Code)
	}

	panel := decode(t, overview)
	if panel["device"].(map[string]any)["online"] != true {
		t.Error("perangkat harus daring setelah kirim pembacaan")
	}

	latest := panel["reading"].(map[string]any)
	if len(latest["sensors"].([]any)) != 2 {
		t.Errorf("sensor tersimpan = %d, mau 2", len(latest["sensors"].([]any)))
	}
}

func TestRejectsMalformedPayload(t *testing.T) {
	env := newEnv(t)
	response := env.request(t, http.MethodPost, "/device/readings", map[string]any{"sensors": []any{}}, deviceHeaders())

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, mau 400", response.Code)
	}
}

func TestFanUsesHysteresisFromServerState(t *testing.T) {
	env := newEnv(t)

	first := decode(t, env.request(t, http.MethodPost, "/device/readings", reading(35), deviceHeaders()))
	if first["state"].(map[string]any)["fan"] != true {
		t.Fatal("fan harus menyala saat suhu 35")
	}

	second := decode(t, env.request(t, http.MethodPost, "/device/readings", reading(30.5), deviceHeaders()))
	if second["state"].(map[string]any)["fan"] != true {
		t.Error("fan harus tetap menyala selama suhu di atas ambang minus histeresis")
	}

	third := decode(t, env.request(t, http.MethodPost, "/device/readings", reading(25), deviceHeaders()))
	if third["state"].(map[string]any)["fan"] != false {
		t.Error("fan harus mati setelah suhu turun di bawah ambang minus histeresis")
	}
}

func TestCommandsDeliveredOnce(t *testing.T) {
	env := newEnv(t)

	queued := env.request(t, http.MethodPost, "/api/commands", map[string]any{"command": "restart"}, nil)
	if queued.Code != http.StatusAccepted {
		t.Fatalf("status = %d, mau 202", queued.Code)
	}

	first := decode(t, env.request(t, http.MethodPost, "/device/readings", reading(27), deviceHeaders()))
	commands := first["commands"].([]any)

	if len(commands) != 1 {
		t.Fatalf("perintah terkirim = %d, mau 1", len(commands))
	}
	if commands[0].(map[string]any)["command"] != "restart" {
		t.Errorf("perintah = %v", commands[0])
	}

	second := decode(t, env.request(t, http.MethodPost, "/device/readings", reading(27), deviceHeaders()))
	if len(second["commands"].([]any)) != 0 {
		t.Error("perintah tidak boleh terkirim dua kali")
	}
}

func TestHistoryAscending(t *testing.T) {
	env := newEnv(t)

	for _, value := range []float64{28, 29, 30} {
		if response := env.request(t, http.MethodPost, "/device/readings", reading(value), deviceHeaders()); response.Code != http.StatusOK {
			t.Fatalf("ingest status = %d", response.Code)
		}
	}

	response := env.request(t, http.MethodGet, "/api/history?limit=10", nil, nil)
	body := decode(t, response)
	readings := body["readings"].([]any)

	if len(readings) != 3 {
		t.Fatalf("riwayat = %d, mau 3", len(readings))
	}

	first := readings[0].(map[string]any)["ts"].(float64)
	last := readings[2].(map[string]any)["ts"].(float64)
	if first > last {
		t.Error("riwayat harus urut naik")
	}
}

func TestSettingsValidatedAndStored(t *testing.T) {
	env := newEnv(t)

	rejected := env.request(t, http.MethodPut, "/api/settings", map[string]any{"b1": 90, "iv": 100}, nil)
	if rejected.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, mau 422", rejected.Code)
	}

	accepted := env.request(t, http.MethodPut, "/api/settings", map[string]any{"b1": 25.5, "hys": 1.5, "iv": 6000}, nil)
	if accepted.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", accepted.Code, accepted.Body.String())
	}

	overview := decode(t, env.request(t, http.MethodGet, "/api/overview", nil, nil))
	settings := overview["settings"].(map[string]any)

	if settings["b1"] != 25.5 || settings["iv"] != 6000.0 {
		t.Errorf("pengaturan tidak tersimpan: %v", settings)
	}

	limits := overview["limits"].(map[string]any)
	if _, ok := limits["bt"]; !ok {
		t.Error("batas bawaan harus tersedia saat perangkat belum kirim meta")
	}
}

func TestUnknownCommandRejected(t *testing.T) {
	env := newEnv(t)
	response := env.request(t, http.MethodPost, "/api/commands", map[string]any{"command": "reboot-now"}, nil)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, mau 422", response.Code)
	}
}

func TestDuplicateDeviceTimestampIgnored(t *testing.T) {
	env := newEnv(t)

	payload := reading(35)
	payload["ts"] = int64(1763520000000)

	for attempt := 0; attempt < 2; attempt++ {
		response := env.request(t, http.MethodPost, "/device/readings", payload, deviceHeaders())
		if response.Code != http.StatusOK {
			t.Fatalf("kiriman %d status = %d, body %s", attempt, response.Code, response.Body.String())
		}
	}

	body := decode(t, env.request(t, http.MethodGet, "/api/history?limit=10", nil, nil))
	readings := body["readings"].([]any)

	if len(readings) != 1 {
		t.Fatalf("riwayat = %d, mau 1 karena timestamp sama", len(readings))
	}
}

func TestAccessFailsClosed(t *testing.T) {
	env := newEnv(t)
	env.cfg.BypassAccess = false
	env.cfg.Access = &config.Access{TeamDomain: "https://contoh.cloudflareaccess.com", Audience: "aud"}

	strict := New(env.cfg, env.store, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	recorder := httptest.NewRecorder()
	strict.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, mau 401", recorder.Code)
	}
}
