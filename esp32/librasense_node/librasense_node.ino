// LibraSense node firmware
//
// The microcontroller only reads sensors, reports them, and applies the
// actuator state returned by the server. No thresholds, no automation and no
// alarm decisions live here. All of that is computed by the Go backend.
//
// Wiring (3 rooms: reading room plus two rack rooms, details in docs/005):
//   DHT11 data      23 (baca), 19 (rak 1), 13 (rak 2)
//   KY-037 analog   34 (baca), 35 (rak 1), 32 (rak 2)   ADC1 only
//   Buzzer          26 (baca), 25 (rak 1), 33 (rak 2)
//   Relay -> 12V fan 27
//   LCD I2C         21 (SDA), 22 (SCL)
//
// KY-037 VCC and the LCD backpack use 3.3 V so the pins stay below 3.6 V.
// GPIO12 is avoided on purpose: it is the MTDI strapping pin.
// GPIO34 to 39 are input only, so they never drive the relay or buzzers.
//
// Copy secrets.example.h to secrets.h and fill it in before flashing.

#include <WiFi.h>
#include <WiFiClientSecure.h>
#include <HTTPClient.h>
#include <WebServer.h>
#include <Preferences.h>
#include <Wire.h>
#include <DHT.h>
#include <time.h>
#include "secrets.h"

#define FIRMWARE_VERSION "3.1.1"
#define META_SCHEMA 1
#define POLL_DEFAULT_MS 5000
#define POLL_MIN_MS 5000
#define POLL_MAX_MS 60000

// Three monitored rooms: one reading room and two rack rooms.
#define RACK_COUNT 3
const char *ROOM_LABEL[RACK_COUNT] = {"Ruang Baca", "Ruang Rak 1", "Ruang Rak 2"};

// ---------- pin map ----------
#define DHTTYPE DHT11
const uint8_t DHT_PIN[RACK_COUNT]    = {23, 19, 13};
const uint8_t SUARA_PIN[RACK_COUNT]  = {34, 35, 32};
const uint8_t BUZZER_PIN[RACK_COUNT] = {26, 25, 33};
#define RELAY_PIN 27          // relay channel switches the 12V fan
#define RELAY_ACTIVE_LOW true // most single-channel modules are active low
#define SDA_PIN 21
#define SCL_PIN 22

const char *AP_SSID = "Perpustakaan-ESP32";
const char *AP_PASS = "12345678";

const char *WIFI_SSID_DEFAULT = LIBRASENSE_WIFI_SSID;
const char *WIFI_PASS_DEFAULT = LIBRASENSE_WIFI_PASS;
const char *API_BASE = LIBRASENSE_API_BASE;
const char *DEVICE_ID = LIBRASENSE_DEVICE_ID;
const char *DEVICE_TOKEN = LIBRASENSE_DEVICE_TOKEN;

// Set false once the API has a certificate the ESP32 can verify.
#define TLS_INSECURE true

// ---------- state ----------
struct AppliedState {
  bool fan = false;
  bool buzzer[RACK_COUNT] = {};
};

struct Command {
  int id = 0;
  String name;
};

Preferences prefs;
WebServer portal(80);
DHT dht[RACK_COUNT] = {
  DHT(DHT_PIN[0], DHTTYPE),
  DHT(DHT_PIN[1], DHTTYPE),
  DHT(DHT_PIN[2], DHTTYPE),
};

String wifiSsid = WIFI_SSID_DEFAULT;
String wifiPass = WIFI_PASS_DEFAULT;
String deviceIp;
String wifiMode;

float suhu[RACK_COUNT]   = {};
float lembap[RACK_COUNT] = {};
int suara[RACK_COUNT]    = {};
bool okSuhu[RACK_COUNT]  = {};
bool okSuara[RACK_COUNT] = {};
AppliedState applied;

uint32_t pollMs = POLL_DEFAULT_MS;
unsigned long lastRead    = 0;
unsigned long lastPush    = 0;
unsigned long lastWifiTry = 0;
unsigned long lastLcd     = 0;
uint8_t pushFailCount = 0;
unsigned long buzzerUntil = 0;
int buzzerIndex = -1;
unsigned long rebootAt = 0;
bool apMode    = false;
bool metaSent  = false;
uint32_t pushCount = 0;

uint8_t lcdAddr = 0;
bool lcdOK = false;
uint8_t lcdPage = 0;

// ---------- LCD ----------
class Lcd {
  uint8_t addr = 0;
  const uint8_t BL = 0x08, EN = 0x04;

  void write4(uint8_t value) {
    Wire.beginTransmission(addr);
    Wire.write(value | BL | EN);
    Wire.endTransmission();
    delayMicroseconds(5);
    Wire.beginTransmission(addr);
    Wire.write((value | BL) & ~EN);
    Wire.endTransmission();
    delayMicroseconds(100);
  }

  void send(uint8_t value, uint8_t rs) {
    write4((value & 0xF0) | rs);
    write4(((value << 4) & 0xF0) | rs);
  }

public:
  void begin(uint8_t address) {
    addr = address;
    delay(50);
    write4(0x30); delay(5);
    write4(0x30); delay(1);
    write4(0x30); delay(1);
    write4(0x20); delay(1);
    send(0x28, 0);
    send(0x0C, 0);
    send(0x01, 0); delay(3);
    send(0x06, 0);
  }

  void setCursor(uint8_t column, uint8_t row) {
    send(0x80 | (column + (row ? 0x40 : 0)), 0);
  }

  void print(const char *text) {
    while (*text) { send((uint8_t)*text++, 1); }
  }

  void line(uint8_t row, const char *text) {
    char buffer[17];
    snprintf(buffer, sizeof(buffer), "%-16s", text);
    setCursor(0, row);
    print(buffer);
  }
};

Lcd lcd;

uint8_t findLcd() {
  const uint8_t candidates[2] = {0x27, 0x3F};
  for (uint8_t i = 0; i < 2; i++) {
    Wire.beginTransmission(candidates[i]);
    if (Wire.endTransmission() == 0) { return candidates[i]; }
  }
  return 0;
}

// ---------- preferences (wifi only) ----------
void loadSettings() {
  prefs.begin("perpus", false);
  wifiSsid = prefs.getString("ws", WIFI_SSID_DEFAULT);
  wifiPass  = prefs.getString("wp", WIFI_PASS_DEFAULT);
}

// ---------- outputs ----------
void driveOutputs() {
  digitalWrite(RELAY_PIN, (applied.fan == RELAY_ACTIVE_LOW) ? LOW : HIGH);

  for (int i = 0; i < RACK_COUNT; i++) {
    const bool pulse = buzzerIndex == i && (long)(buzzerUntil - millis()) > 0;
    digitalWrite(BUZZER_PIN[i], (applied.buzzer[i] || pulse) ? HIGH : LOW);
  }
}

// ---------- JSON helpers ----------
String jsonEscape(const String &value) {
  String out;
  out.reserve(value.length() + 8);
  for (size_t i = 0; i < value.length(); i++) {
    const char c = value[i];
    switch (c) {
      case '"':  out += F("\\\""); break;
      case '\\': out += F("\\\\"); break;
      case '\n': out += F("\\n");  break;
      case '\r': out += F("\\r");  break;
      case '\t': out += F("\\t");  break;
      default:
        if ((uint8_t)c < 0x20) {
          char buffer[7];
          snprintf(buffer, sizeof(buffer), "\\u%04x", (uint8_t)c);
          out += buffer;
        } else {
          out += c;
        }
    }
  }
  return out;
}

int findKey(const String &json, const String &key, int from = 0) {
  return json.indexOf("\"" + key + "\":", from);
}

bool jsonNumber(const String &json, const String &key, double &out) {
  const int at = findKey(json, key);
  if (at < 0) { return false; }
  const char *raw = json.c_str() + at + key.length() + 3;
  char *end = nullptr;
  const double value = strtod(raw, &end);
  if (end == raw) { return false; }
  out = value;
  return true;
}

bool jsonBool(const String &json, const String &key, bool &out) {
  const int at = findKey(json, key);
  if (at < 0) { return false; }
  const int start = at + key.length() + 3;
  if (json.startsWith("true",  start)) { out = true;  return true; }
  if (json.startsWith("false", start)) { out = false; return true; }
  return false;
}

bool jsonString(const String &json, const String &key, String &out) {
  const int at = findKey(json, key);
  if (at < 0) { return false; }
  const int open = json.indexOf('"', at + key.length() + 3);
  if (open < 0) { return false; }
  String result;
  for (int i = open + 1; i < (int)json.length(); i++) {
    const char c = json[i];
    if (c == '\\' && i + 1 < (int)json.length()) { result += json[++i]; continue; }
    if (c == '"') { out = result; return true; }
    result += c;
  }
  return false;
}

int objectEnd(const String &json, int start) {
  int depth = 0;
  bool inString = false;
  for (int i = start; i < (int)json.length(); i++) {
    const char c = json[i];
    if (inString) {
      if (c == '\\') { i++; } else if (c == '"') { inString = false; }
      continue;
    }
    if (c == '"')      { inString = true; }
    else if (c == '{') { depth++; }
    else if (c == '}') { depth--; if (depth == 0) { return i; } }
  }
  return -1;
}

// ---------- sensors ----------
int readNoise(uint8_t pin, bool &present) {
  int low = 4095, high = 0;
  int count = 0;
  const unsigned long started = millis();
  while (millis() - started < 50) {
    const int value = analogRead(pin);
    count++;
    if (value < low)  low  = value;
    if (value > high) high = value;
  }
  // KY-037 modules differ: some idle near 0 V, others near VCC/2. Judge
  // presence from the whole window, not the DC offset, so a module with a
  // low baseline is not reported as missing. Only a pin stuck on a rail for
  // the entire window counts as absent.
  present = count > 0 && high > 0 && low < 4095;
  return high - low;
}

void readSensors() {
  for (int i = 0; i < RACK_COUNT; i++) {
    suhu[i]   = dht[i].readTemperature();
    lembap[i] = dht[i].readHumidity();
    okSuhu[i] = !isnan(suhu[i]) && !isnan(lembap[i]);
    suara[i]  = readNoise(SUARA_PIN[i], okSuara[i]);
  }
}

// Keeps the LCD readable when a sensor fails instead of printing nan.
void formatReading(char *buffer, size_t size, bool ok, float value) {
  if (ok) {
    snprintf(buffer, size, "%.1f", value);
  } else {
    snprintf(buffer, size, "--");
  }
}

// ---------- HTTP ----------
bool apiRequest(const String &path, const String &body, String &response) {
  const String url = String(API_BASE) + path;
  HTTPClient http;
  WiFiClientSecure secure;
  WiFiClient plain;
  bool started = false;

  if (url.startsWith("https://")) {
    if (TLS_INSECURE) { secure.setInsecure(); }
    started = http.begin(secure, url);
  } else {
    started = http.begin(plain, url);
  }

  if (!started) { return false; }

  http.setTimeout(8000);
  http.setReuse(false);
  http.addHeader("Content-Type", "application/json");
  http.addHeader("Authorization", String("Bearer ") + DEVICE_TOKEN);

  const int status = http.POST(body);
  response = status > 0 ? http.getString() : String();
  http.end();

  if (status < 200 || status >= 300) {
    Serial.printf("API %s -> %d\n", path.c_str(), status);
    if (response.length() > 0) { Serial.println(response); }
    return false;
  }
  return true;
}

void sendAck(int id) {
  if (id <= 0) { return; }
  String response;
  apiRequest("/device/ack", String("{\"ids\":[") + id + "]}", response);
}

// ---------- payloads ----------
String buildMeta() {
  String body = F("{\"schema\":");
  body += META_SCHEMA;
  body += F(",\"firmware\":\"");
  body += FIRMWARE_VERSION;
  body += F("\",\"poll_ms\":");
  body += pollMs;
  body += F(",\"history_depth\":60,\"limits\":{\"bt\":[0,60],\"hys\":[0,10],\"bs\":[0,4095],\"iv\":[5000,60000]}");
  body += F(",\"sensors\":[");

  bool first = true;
  for (int i = 0; i < RACK_COUNT; i++) {
    if (!first) body += ',';
    first = false;
    body += F("{\"id\":\"dht");
    body += i + 1;
    body += F("\",\"type\":\"temperature\",\"label\":\"");
    body += ROOM_LABEL[i];
    body += F("\",\"rack\":");
    body += i + 1;
    body += '}';
  }
  for (int i = 0; i < RACK_COUNT; i++) {
    body += F(",{\"id\":\"hum");
    body += i + 1;
    body += F("\",\"type\":\"humidity\",\"label\":\"Kelembapan ");
    body += ROOM_LABEL[i];
    body += F("\",\"rack\":");
    body += i + 1;
    body += '}';
  }
  for (int i = 0; i < RACK_COUNT; i++) {
    body += F(",{\"id\":\"sound");
    body += i + 1;
    body += F("\",\"type\":\"noise\",\"label\":\"Kebisingan ");
    body += ROOM_LABEL[i];
    body += F("\",\"rack\":");
    body += i + 1;
    body += '}';
  }

  body += "]}";
  return body;
}

String buildReadings() {
  String body = F("{\"firmware\":\"");
  body += FIRMWARE_VERSION;
  body += F("\",\"ip\":\"");
  body += deviceIp;
  body += F("\",\"mode\":\"");
  body += wifiMode;
  body += '"';

  const time_t now = time(nullptr);
  if (now > 1700000000) {
    body += F(",\"ts\":");
    body += (unsigned long long)now * 1000ULL;
  }

  body += F(",\"applied\":{\"fan\":");
  body += applied.fan ? F("true") : F("false");
  body += F(",\"relay\":");
  body += applied.fan ? F("true") : F("false");
  body += F(",\"buzzer\":[");
  for (int i = 0; i < RACK_COUNT; i++) {
    if (i > 0) body += ',';
    body += applied.buzzer[i] ? F("true") : F("false");
  }
  body += F("],\"uptime\":");
  body += millis() / 1000;
  body += '}';

  body += F(",\"sensors\":[");
  for (int i = 0; i < RACK_COUNT; i++) {
    if (i > 0) body += ',';
    body += F("{\"id\":\"dht");
    body += i + 1;
    body += F("\",\"type\":\"temperature\",\"unit\":\"C\",\"rack\":");
    body += i + 1;
    body += F(",\"ok\":");
    body += okSuhu[i] ? F("true") : F("false");
    body += F(",\"value\":");
    body += okSuhu[i] ? String(suhu[i], 1) : String("null");
    body += '}';
  }
  for (int i = 0; i < RACK_COUNT; i++) {
    body += F(",{\"id\":\"hum");
    body += i + 1;
    body += F("\",\"type\":\"humidity\",\"unit\":\"%\",\"rack\":");
    body += i + 1;
    body += F(",\"ok\":");
    body += okSuhu[i] ? F("true") : F("false");
    body += F(",\"value\":");
    body += okSuhu[i] ? String(lembap[i], 0) : String("null");
    body += '}';
  }
  for (int i = 0; i < RACK_COUNT; i++) {
    body += F(",{\"id\":\"sound");
    body += i + 1;
    body += F("\",\"type\":\"noise\",\"unit\":\"adc\",\"rack\":");
    body += i + 1;
    body += F(",\"ok\":");
    body += okSuara[i] ? F("true") : F("false");
    body += F(",\"value\":");
    body += suara[i];
    body += '}';
  }

  body += ']';

  if (!metaSent || pushCount % 60 == 0) {
    body += F(",\"meta\":");
    body += buildMeta();
  }

  body += '}';
  return body;
}

// ---------- server state and commands ----------
void applyServerState(const String &response) {
  const int at = findKey(response, "state");
  if (at < 0) { return; }
  const int start = response.indexOf('{', at);
  if (start < 0) { return; }
  const int end = objectEnd(response, start);
  if (end < 0) { return; }

  const String state = response.substring(start, end + 1);
  bool fan = false;
  bool buzzer[RACK_COUNT] = {};
  double interval = 0;

  if (jsonBool(state, "fan", fan)) { applied.fan = fan; }

  const int list = state.indexOf("\"buzzer\":[");
  if (list >= 0) {
    int index = 0;
    int cursor = list + 10;
    while (index < RACK_COUNT && cursor < (int)state.length()) {
      while (cursor < (int)state.length() && (state[cursor] == ' ' || state[cursor] == ',')) { cursor++; }
      if (state.startsWith("true",  cursor)) { buzzer[index++] = true;  cursor += 4; }
      else if (state.startsWith("false", cursor)) { buzzer[index++] = false; cursor += 5; }
      else { break; }
    }
    for (int i = 0; i < RACK_COUNT; i++) { applied.buzzer[i] = buzzer[i]; }
  }

  if (jsonNumber(state, "poll_ms", interval)) {
    const double clamped = interval < POLL_MIN_MS ? POLL_MIN_MS : (interval > POLL_MAX_MS ? POLL_MAX_MS : interval);
    pollMs = (uint32_t)clamped;
  }

  driveOutputs();
}

void handleCommands(const String &response) {
  const int list = response.indexOf("\"commands\":[");
  if (list < 0) { return; }
  int cursor = list + 12;

  for (int guard = 0; guard < 8; guard++) {
    const int start = response.indexOf('{', cursor);
    if (start < 0) { return; }
    const int end = objectEnd(response, start);
    if (end < 0) { return; }

    const String item = response.substring(start, end + 1);
    cursor = end + 1;

    Command command;
    double id = 0;
    if (jsonNumber(item, "id", id)) { command.id = (int)id; }
    if (!jsonString(item, "command", command.name)) { continue; }

    Serial.printf("CMD %s (%d)\n", command.name.c_str(), command.id);

    if (command.name == "restart") {
      rebootAt = millis() + 300;
    } else if (command.name == "reset") {
      prefs.clear();
      rebootAt = millis() + 300;
    } else if (command.name.length() == 5 && command.name.startsWith("test")) {
      const int index = command.name.charAt(4) - '1';
      if (index >= 0 && index < RACK_COUNT) {
        buzzerIndex = index;
        buzzerUntil = millis() + 400;
        driveOutputs();
      }
    }

    sendAck(command.id);
  }
}

// Backoff keeps the node from hammering a server that is down.
// The sensor and LCD loop keeps running on the normal interval.
unsigned long pushInterval() {
  switch (pushFailCount) {
    case 0:  return pollMs;
    case 1:  return pollMs + 5000UL;
    case 2:  return pollMs + 10000UL;
    case 3:  return pollMs + 20000UL;
    case 4:  return pollMs + 40000UL;
    default: return pollMs + 60000UL;
  }
}

void pushReadings() {
  String response;
  const bool ok = apiRequest("/device/readings", buildReadings(), response);
  pushCount++;
  if (!ok) {
    if (pushFailCount < 5) { pushFailCount++; }
    return;
  }
  pushFailCount = 0;
  metaSent = true;
  applyServerState(response);
  handleCommands(response);
}

// ---------- WiFi ----------
void connectWifi() {
  if (WiFi.status() == WL_CONNECTED) { return; }
  if (millis() - lastWifiTry < 5000) { return; }
  lastWifiTry = millis();
  WiFi.mode(WIFI_STA);
  WiFi.begin(wifiSsid.c_str(), wifiPass.c_str());
  const unsigned long started = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - started < 8000) { delay(100); }
  if (WiFi.status() == WL_CONNECTED) {
    deviceIp = WiFi.localIP().toString();
    wifiMode = "WiFi";
    pushFailCount = 0;
    configTime(0, 0, "pool.ntp.org", "time.google.com");
    Serial.printf("WiFi tersambung: %s\n", deviceIp.c_str());
  } else {
    Serial.println("WiFi gagal, mencoba lagi");
  }
}

void startPortal() {
  apMode = true;
  WiFi.mode(WIFI_AP);
  WiFi.softAP(AP_SSID, AP_PASS);
  deviceIp = WiFi.softAPIP().toString();
  wifiMode  = "Access Point";

  portal.on("/", HTTP_GET, []() {
    String page = F("<!DOCTYPE html><html lang=\"id\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">"
                    "<title>LibraSense</title><style>body{font-family:system-ui;max-width:420px;margin:32px auto;padding:0 16px}"
                    "input,button{width:100%;padding:10px;margin:6px 0;font:inherit}button{background:#0f766e;color:#fff;border:0;border-radius:8px}"
                    "label{font-size:.85rem;color:#555}</style><h1>LibraSense</h1>"
                    "<form method=\"post\" action=\"/save\"><label>Nama WiFi</label><input name=\"ssid\" required>"
                    "<label>Sandi WiFi</label><input name=\"pass\" type=\"password\">"
                    "<button>Simpan dan mulai ulang</button></form></html>");
    portal.send(200, "text/html", page);
  });

  portal.on("/save", HTTP_POST, []() {
    if (portal.hasArg("ssid") && portal.arg("ssid").length() > 0) {
      prefs.begin("perpus", false);
      prefs.putString("ws", portal.arg("ssid"));
      prefs.putString("wp", portal.arg("pass"));
      portal.send(200, "text/html", "<p>Pengaturan disimpan. Perangkat akan mulai ulang.</p>");
      rebootAt = millis() + 800;
    } else {
      portal.send(400, "text/plain", "SSID wajib diisi");
    }
  });

  portal.begin();
  Serial.printf("Portal konfigurasi: http://%s\n", deviceIp.c_str());
}

// ---------- lifecycle ----------
void setup() {
  Serial.begin(115200);
  Wire.begin(SDA_PIN, SCL_PIN);
  Wire.setClock(100000);

  loadSettings();

  pinMode(RELAY_PIN, OUTPUT);
  digitalWrite(RELAY_PIN, RELAY_ACTIVE_LOW ? HIGH : LOW);

  for (int i = 0; i < RACK_COUNT; i++) {
    pinMode(BUZZER_PIN[i], OUTPUT);
    digitalWrite(BUZZER_PIN[i], LOW);
    pinMode(SUARA_PIN[i], INPUT);
    dht[i].begin();
  }

  lcdAddr = findLcd();
  if (lcdAddr) {
    lcdOK = true;
    lcd.begin(lcdAddr);
    lcd.line(0, "LibraSense 3.1");
    lcd.line(1, "Menyambung WiFi");
  }

  WiFi.mode(WIFI_STA);
  WiFi.setAutoReconnect(true);
  WiFi.begin(wifiSsid.c_str(), wifiPass.c_str());

  const unsigned long started = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - started < 15000) { delay(150); }

  if (WiFi.status() == WL_CONNECTED) {
    deviceIp = WiFi.localIP().toString();
    wifiMode  = "WiFi";
    configTime(0, 0, "pool.ntp.org", "time.google.com");
    Serial.printf("WiFi tersambung: %s\n", deviceIp.c_str());
  } else {
    startPortal();
  }
}

void loop() {
  if (apMode) {
    portal.handleClient();
  } else {
    connectWifi();
  }

  if (rebootAt > 0 && millis() >= rebootAt) { ESP.restart(); }

  const unsigned long now = millis();

  if (now - lastRead >= pollMs) {
    lastRead = now;
    readSensors();
    Serial.printf("Suhu %.1f %.1f %.1f | Suara %d %d %d\n",
                  suhu[0], suhu[1], suhu[2], suara[0], suara[1], suara[2]);
  }

  if (buzzerIndex >= 0 && (long)(buzzerUntil - now) <= 0) {
    buzzerIndex = -1;
    driveOutputs();
  }

  if (!apMode && now - lastPush >= pushInterval() && WiFi.status() == WL_CONNECTED) {
    lastPush = now;
    pushReadings();
  }

  if (lcdOK && now - lastLcd >= 2000) {
    lastLcd = now;
    char row0[17];
    char row1[17];
    char t1[8];
    char t2[8];
    char t3[8];

    if (lcdPage == 0) {
      formatReading(t1, sizeof(t1), okSuhu[0], suhu[0]);
      formatReading(t2, sizeof(t2), okSuhu[1], suhu[1]);
      formatReading(t3, sizeof(t3), okSuhu[2], suhu[2]);
      snprintf(row0, sizeof(row0), "D1:%s D2:%s", t1, t2);
      snprintf(row1, sizeof(row1), "D3:%s F:%s", t3, applied.fan ? "ON" : "OFF");
    } else if (lcdPage == 1) {
      snprintf(row0, sizeof(row0), "N1:%d N2:%d", suara[0], suara[1]);
      snprintf(row1, sizeof(row1), "N3:%d %s", suara[2], WiFi.status() == WL_CONNECTED ? "On" : "Off");
    } else {
      snprintf(row0, sizeof(row0), "%s", wifiMode.c_str());
      snprintf(row1, sizeof(row1), "%s", deviceIp.c_str());
    }

    lcd.line(0, row0);
    lcd.line(1, row1);
    lcdPage = (lcdPage + 1) % 3;
  }
}
