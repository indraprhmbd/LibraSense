/* ============================================================
   app.js - LibraSense panel
   Alpine.js, plain JS, no build step.

   Modules (each returns a plain object):
     api        - fetch wrapper, single source of truth for HTTP
     store      - reactive state store, shared across components
     panelApp   - root Alpine component (x-data="panelApp()")
     rackCard   - per-rack derived data helper
     chartSvg   - SVG temperature chart builder
   ============================================================ */

'use strict';

/* ============================================================
   api - HTTP layer
   All requests go through here. Throws Error on non-2xx.
   ============================================================ */
const api = (() => {
  async function request(path, options = {}) {
    const headers = { Accept: 'application/json' };
    if (options.body !== undefined) {
      headers['Content-Type'] = 'application/json';
    }
    const res = await fetch(path, {
      credentials: 'same-origin',
      ...options,
      headers,
    });
    const text = await res.text();
    let data = null;
    try { data = text ? JSON.parse(text) : null; } catch { /* non-JSON */ }
    if (!res.ok) {
      const detail =
        data && Array.isArray(data.issues) && data.issues.length > 0
          ? ': ' + data.issues.join(', ')
          : '';
      throw new Error(
        (data && data.error ? data.error : 'HTTP ' + res.status) + detail
      );
    }
    return data;
  }

  return {
    overview: () => request('/api/overview'),
    history:  (limit = 120) => request(`/api/history?limit=${limit}`),
    settings: (payload) =>
      request('/api/settings', { method: 'PUT', body: JSON.stringify(payload) }),
    command:  (cmd) =>
      request('/api/commands', { method: 'POST', body: JSON.stringify({ command: cmd }) }),
  };
})();

/* ============================================================
   store - single reactive state object
   Passed into Alpine via x-data spread or reference.
   ============================================================ */
const store = (() => {
  // Internal mutable state - Alpine wraps this reactively in panelApp
  return {
    overview: null,
    history:  [],
    // Rolling noise trace per rack, drawn as a signal line in the floor plan.
    noiseTrace: { 1: [], 2: [], 3: [] },
    _noiseSeeded: false,
    error:    '',
    busy:     false,
    toast:    null,
    _toastTimer: null,
    tab:      'dasbor',

    // --- data accessors ---

    device() {
      return this.overview ? this.overview.device : null;
    },

    reading() {
      return this.overview ? this.overview.reading : null;
    },

    settings() {
      return this.overview && this.overview.settings ? this.overview.settings : {};
    },

    limits() {
      return this.overview && this.overview.limits ? this.overview.limits : {};
    },

    actuators() {
      return this.overview ? this.overview.actuators : null;
    },

    status() {
      return this.overview ? this.overview.status : { hot: [], noisy: [] };
    },

    pending() {
      return this.overview ? this.overview.pending : 0;
    },

    online() {
      const d = this.device();
      return d ? d.online : false;
    },

    sensors() {
      const r = this.reading();
      return r && Array.isArray(r.sensors) ? r.sensors : [];
    },

    desired() {
      const a = this.actuators();
      return a ? a.desired : null;
    },

    applied() {
      const a = this.actuators();
      return a ? a.applied : null;
    },

    // --- mutations ---

    setOverview(data) {
      this.overview = data;
      this.error = '';
      this.pushNoise();
    },

    setHistory(data) {
      this.history = Array.isArray(data.readings) ? data.readings : [];
      this.seedNoise();
    },

    /**
     * Append the newest noise value per rack to the trace buffer.
     * Skipped when the sensor is missing so the line does not drop to zero.
     */
    pushNoise() {
      const sensors = this.sensors();
      for (const rack of [1, 2, 3]) {
        const s = sensors.find(
          (x) => x.type === 'noise' && x.rack === rack && x.ok !== false
        );
        if (!s || typeof s.value !== 'number') continue;
        const buf = this.noiseTrace[rack];
        buf.push(s.value);
        if (buf.length > 40) buf.shift();
      }
    },

    /** Fill the trace from stored history once, so the line is not empty on load. */
    seedNoise() {
      if (this._noiseSeeded || !this.history.length) return;
      this._noiseSeeded = true;
      const rows = this.history.slice(-40);
      rows.forEach((row, i) => {
        const sensors = Array.isArray(row.sensors) ? row.sensors : [];
        for (const rack of [1, 2, 3]) {
          const s = sensors.find(
            (x) => x.type === 'noise' && x.rack === rack && x.ok !== false
          );
          if (!s || typeof s.value !== 'number') continue;
          if (!this.noiseTrace[rack][i]) this.noiseTrace[rack][i] = null;
          this.noiseTrace[rack][i] = s.value;
        }
      });
      for (const rack of [1, 2, 3]) {
        this.noiseTrace[rack] = this.noiseTrace[rack].filter(
          (v) => typeof v === 'number'
        )
          .slice(-40);
      }
    },

    setError(msg) {
      this.error = msg;
    },

    notify(text, isError = false) {
      this.toast = { text, isError };
      clearTimeout(this._toastTimer);
      this._toastTimer = setTimeout(() => { this.toast = null; }, 3400);
    },
  };
})();

/* ============================================================
   rackCard - derives rack display data from store
   ============================================================ */
const rackCard = {
  /**
   * Returns an array of rack objects derived from overview data.
   * Uses meta.sensors when available (richer labels),
   * falls back to raw reading sensors.
   *
   * @param {object} st - store instance
   * @returns {Array<{rack, id, label, temperature, humidity, noise, ok, hot, noisy}>}
   */
  list(st) {
    const meta =
      st.overview &&
      st.overview.device.meta &&
      Array.isArray(st.overview.device.meta.sensors)
        ? st.overview.device.meta.sensors
        : [];

    const declared = meta.filter(
      (s) => s.type === 'temperature' && s.rack
    );
    const sensors = st.sensors().filter((s) => s.type === 'temperature');

    const base =
      declared.length > 0
        ? declared.map((s) => ({
            rack:  s.rack,
            id:    s.id,
            label: s.label || 'Ruang ' + s.rack,
          }))
        : sensors.map((s, i) => ({
            rack:  s.rack || i + 1,
            id:    s.id,
            label: s.label || 'Ruang ' + (s.rack || i + 1),
          }));

    const hot   = st.status().hot;
    const noisy = st.status().noisy;

    return base
      .sort((a, b) => a.rack - b.rack)
      .map((item) => {
        const temp = rackCard.sensorValue(st, item.rack, 'temperature');
        return {
          ...item,
          temperature: temp,
          humidity:    rackCard.sensorValue(st, item.rack, 'humidity'),
          noise:       rackCard.sensorValue(st, item.rack, 'noise'),
          ok:          temp !== null,
          hot:         Array.isArray(hot)   && hot.indexOf(item.rack)   >= 0,
          noisy:       Array.isArray(noisy) && noisy.indexOf(item.rack) >= 0,
          threshold:   rackCard.threshold(st, item.rack),
        };
      });
  },

  sensorValue(st, rack, type) {
    const s = st.sensors().find((x) => x.rack === rack && x.type === type);
    return s && s.ok && typeof s.value === 'number' ? s.value : null;
  },

  threshold(st, rack) {
    const cfg = st.settings();
    return cfg['b' + rack] ?? cfg.b1 ?? 30;
  },

  barWidth(temp) {
    if (temp === null || typeof temp !== 'number') return 0;
    return Math.min(100, Math.max(0, (temp / 50) * 100));
  },
};

/* ============================================================
   chartSvg - builds an inline SVG temperature history chart
   ============================================================ */
const chartSvg = {
  COLORS: ['#0d9488', '#f97316', '#6366f1'],

  /**
   * @param {Array} rows  - history readings
   * @param {Array} racks - rack objects from rackCard.list()
   * @returns {string} HTML string (svg + legend div)
   */
  build(rows, racks) {
    if (!rows.length) {
      return '<p class="text-muted text-sm">Belum ada riwayat pembacaan.</p>';
    }

    const series = racks.slice(0, 3).map((rack, i) => ({
      rack,
      color: chartSvg.COLORS[i % chartSvg.COLORS.length],
      values: rows.map((row) => {
        const s = Array.isArray(row.sensors)
          ? row.sensors.find((x) => x.id === rack.id)
          : null;
        return s && s.ok && typeof s.value === 'number' ? s.value : null;
      }),
    }));

    const flat = series.flatMap((s) => s.values.filter((v) => v !== null));
    if (!flat.length) {
      return '<p class="text-muted text-sm">Belum ada nilai suhu tercatat.</p>';
    }

    const W = 600, H = 160, PAD_T = 16, PAD_B = 16;
    const plotH = H - PAD_T - PAD_B;
    const minV  = Math.floor(Math.min(...flat) - 1);
    const maxV  = Math.ceil(Math.max(...flat) + 1);
    const span  = Math.max(1, maxV - minV);
    const count = Math.max(1, rows.length - 1);

    const px = (i) => ((i * W) / count).toFixed(1);
    const py = (v) => (PAD_T + plotH - ((v - minV) / span) * plotH).toFixed(1);

    // Grid lines
    const gridLines = [0, 1, 2, 3]
      .map((n) => {
        const y = PAD_T + (n * plotH) / 3;
        const lv = (maxV - (n * span) / 3).toFixed(1);
        return (
          `<line x1="0" x2="${W}" y1="${y}" y2="${y}" stroke="currentColor" stroke-opacity=".1"/>` +
          `<text x="4" y="${y - 3}" font-size="10" fill="currentColor" opacity=".45">${lv}</text>`
        );
      })
      .join('');

    // Polylines
    const polylines = series
      .map(({ values, color }) => {
        const pts = values
          .map((v, i) => (v === null ? null : `${px(i)},${py(v)}`))
          .filter(Boolean)
          .join(' ');
        return pts
          ? `<polyline fill="none" stroke="${color}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" points="${pts}"/>`
          : '';
      })
      .join('');

    // Legend
    const legend = series
      .map(
        ({ rack, color }) =>
          `<span class="chart-legend-item"><span class="chart-legend-dot" style="background:${color}"></span>${rack.label}</span>`
      )
      .join('');

    return (
      `<svg viewBox="0 0 ${W} ${H}" role="img" aria-label="Grafik suhu per ruang" style="color:var(--ink)">` +
      gridLines + polylines +
      `</svg>` +
      `<div class="chart-legend">${legend}<span class="text-muted">${rows.length} pembacaan</span></div>`
    );
  },
};

/* ============================================================
   formSync - converts settings <-> form object
   ============================================================ */
const formSync = {
  fromSettings(cfg) {
    const bm = Number(cfg.bm ?? 7); // default: all three rooms enabled
    return {
      b1:  cfg.b1  ?? 30,
      b2:  cfg.b2  ?? 30,
      b3:  cfg.b3  ?? 30,
      hys: cfg.hys ?? 1,
      bs:  cfg.bs  ?? 1000,
      iv:  cfg.iv  ?? 5000,
      fm:  cfg.fm  ?? 0,
      al:  cfg.al  !== false,
      na:  cfg.na  === true,
      m1:  (bm & 1) === 1,
      m2:  (bm & 2) === 2,
      m3:  (bm & 4) === 4,
    };
  },

  toPayload(form) {
    const bm =
      (form.m1 ? 1 : 0) |
      (form.m2 ? 2 : 0) |
      (form.m3 ? 4 : 0);
    return {
      b1:  Number(form.b1),
      b2:  Number(form.b2),
      b3:  Number(form.b3),
      hys: Number(form.hys),
      bs:  Number(form.bs),
      iv:  Number(form.iv),
      fm:  Number(form.fm),
      bm,
      al:  Boolean(form.al),
      na:  Boolean(form.na),
    };
  },
};

/* ============================================================
   time helpers
   ============================================================ */
const timeHelper = {
  clock(ts) {
    if (!ts) return 'belum pernah';
    return new Date(ts).toLocaleTimeString('id-ID', {
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    });
  },

  stamp(ts) {
    if (!ts) return '';
    return new Date(ts).toLocaleString('id-ID', {
      day: '2-digit', month: 'short',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    });
  },

  relative(ts) {
    if (!ts) return '';
    const diff = Math.round((Date.now() - ts) / 1000);
    if (diff < 5)   return 'baru saja';
    if (diff < 60)  return diff + ' detik lalu';
    if (diff < 3600) return Math.floor(diff / 60) + ' menit lalu';
    return timeHelper.clock(ts);
  },
};

/* ============================================================
   panelApp - root Alpine component
   ============================================================ */
function panelApp() {
  return {
    // --- state (mixed into Alpine reactive proxy) ---
    ...store,
    form:   {},
    loaded: false,
    _refreshTimer: null,

    // Commands shown in the UI (rack 3 test shown but harmless if no hardware)
    commands: [
      { command: 'restart', label: 'Mulai ulang' },
      { command: 'reset',   label: 'Setel ulang WiFi' },
      { command: 'test1',   label: 'Uji buzzer Ruang Baca' },
      { command: 'test2',   label: 'Uji buzzer Ruang Rak 1' },
      { command: 'test3',   label: 'Uji buzzer Ruang Rak 2' },
    ],

    // --- lifecycle ---

    async init() {
      await this.refresh();
      this._refreshTimer = setInterval(() => this.refresh(), 5000);
    },

    destroy() {
      clearInterval(this._refreshTimer);
      clearTimeout(this._toastTimer);
    },

    // --- data loading ---

    async refresh() {
      try {
        const [ov, hist] = await Promise.all([
          api.overview(),
          api.history(120),
        ]);
        this.setOverview(ov);
        this.setHistory(hist);
        if (!this.loaded) {
          this.form = formSync.fromSettings(this.settings());
          this.loaded = true;
        }
      } catch (err) {
        this.setError(err.message);
      }
    },

    // --- derived data (called from templates) ---

    racks() {
      return rackCard.list(this);
    },

    rack(num) {
      const list = this.racks();
      return (
        list.find((r) => r.rack === num) || {
          rack: num,
          label: 'Ruang ' + num,
          temperature: null,
          humidity: null,
          noise: null,
          ok: false,
          hot: false,
          noisy: false,
        }
      );
    },

    /**
     * Polyline points for the noise signal strip drawn inside a floor plan HUD.
     * Coordinates are local to a 155x14 box; the caller translates the group.
     */
    noisePoints(num) {
      const buf = this.noiseTrace[num] || [];
      if (buf.length < 2) return '';
      const W = 155;
      const H = 14;
      const pad = 1;
      const peak = Math.max(60, ...buf);
      const step = W / (buf.length - 1);
      return buf
        .map((v, i) => {
          const x = i * step;
          const y = H - pad - (v / peak) * (H - pad * 2);
          return x.toFixed(1) + ',' + y.toFixed(1);
        })
        .join(' ');
    },

    /** Normalized position of the latest noise sample, 0 to 1, for the pulse dot. */
    noiseLevel(num) {
      const buf = this.noiseTrace[num] || [];
      if (!buf.length) return 0;
      const peak = Math.max(60, ...buf);
      return Math.min(1, buf[buf.length - 1] / peak);
    },

    barWidth(temp) {
      return rackCard.barWidth(temp);
    },

    confirmed() {
      const d = this.desired();
      const a = this.applied();
      if (!d || !a) return false;
      if (Boolean(d.fan) !== Boolean(a.fan)) return false;
      const want = Array.isArray(d.buzzer) ? d.buzzer : [];
      const have = Array.isArray(a.buzzer) ? a.buzzer : [];
      return (
        want.length === have.length &&
        want.every((v, i) => Boolean(v) === Boolean(have[i]))
      );
    },

    buzzerText() {
      const d = this.desired();
      if (!d || !Array.isArray(d.buzzer)) return '-';
      const rooms = this.racks().map((rack) => rack.label);
      const active = d.buzzer
        .map((on, i) => (on ? rooms[i] || 'Ruang ' + (i + 1) : null))
        .filter(Boolean);
      return active.length ? active.join(', ') : 'Tidak ada';
    },

    fanText() {
      const d = this.desired();
      if (!d) return '-';
      return d.fan ? 'Nyala' : 'Mati';
    },

    chartHtml() {
      return chartSvg.build(this.history, this.racks());
    },

    limitEntries() {
      return Object.entries(this.limits()).map(([key, range]) => ({ key, range }));
    },

    // --- time helpers (exposed to templates) ---
    clock: timeHelper.clock,
    stamp: timeHelper.stamp,
    relative: timeHelper.relative,

    // --- actions ---

    async save() {
      this.busy = true;
      try {
        await api.settings(formSync.toPayload(this.form));
        this.notify('Pengaturan tersimpan di server');
        await this.refresh();
      } catch (err) {
        this.notify(err.message, true);
      } finally {
        this.busy = false;
      }
    },

    async send(command) {
      this.busy = true;
      try {
        await api.command(command);
        this.notify('Perintah dikirim, menunggu perangkat');
        await this.refresh();
      } catch (err) {
        this.notify(err.message, true);
      } finally {
        this.busy = false;
      }
    },
  };
}
