// Demo en línea: reemplaza la API local de LJ Buddy por un motor simulado en
// el navegador. Nada de lo que se ve aquí sale por un puerto real.
(() => {
  const t0 = Date.now();
  const stamp = () => new Date().toTimeString().slice(0, 8);
  const log = [];
  const say = m => { log.push(stamp() + ' ' + m); if (log.length > 200) log.shift(); };
  const S = {
    listenIP: '0.0.0.0', sourceIP: '', firstUniverse: 0,
    localIPs: ['127.0.0.1', '192.168.1.20', '2.0.0.20'],
    senders: ['2.0.0.10'],
    incoming: [0, 1, 2, 3, 7],
    others: [{ name: 'COM9', desc: 'USB-SERIAL CH340' }],
    devs: [
      { id: '0403:6001:AL05K3XQ', name: 'FT232R USB UART', port: 'COM3', serial: 'AL05K3XQ', kind: 'opendmx', kindLabel: 'Open DMX', detail: '', universes: [0], hz: 36 },
      { id: '0403:6001:EN172839', name: 'DMX USB PRO', port: 'COM4', serial: 'EN172839', kind: 'pro', kindLabel: 'DMX USB Pro', detail: 'N/S 02217283', universes: [1], hz: 40 },
      { id: '0403:6001:6A0F21B3', name: 'ultraDMX Pro', port: 'COM5', serial: '6A0F21B3', kind: 'ultrapro', kindLabel: 'DMXking ultraDMX Pro', detail: 'DMXking ultraDMX Pro N/S 6A0F21B3', universes: [2, 3], hz: 40 },
      { id: '0403:6001:AL01B7YZ', name: 'FT232R USB UART', port: '', serial: '', kind: '', kindLabel: 'Sin identificar', detail: '', universes: [4], hz: 0, absent: true },
    ].map(d => ({ enabled: true, test: false, kindMode: '', ...d })),
  };
  say('LJ Buddy 1.0.0 iniciado. Panel: http://127.0.0.1:6455/');
  say('Art-Net: escuchando en 0.0.0.0:6454');
  for (const d of S.devs) if (!d.absent) {
    say(`Interfaz detectada: ${d.name} en ${d.port}`);
    say(`${d.port} (${d.name}) lista: ${d.kindLabel} → universos [${d.universes.join(' ')}]`);
  }
  say('Art-Net: nuevo emisor 2.0.0.10');

  const accepted = () => !S.sourceIP || S.senders.includes(S.sourceIP);
  const live = u => accepted() && S.incoming.includes(u);
  const jitter = n => n + Math.round(Math.sin(Date.now() / 700 + n) * 1);
  const running = d => !d.absent && d.enabled;

  function levels(u) {
    const t = (Date.now() - t0) / 1000, out = new Uint8Array(512);
    if (!live(u)) return out;
    for (let i = 0; i < 512; i++) {
      let v;
      switch (u % 4) {
        case 0: v = 127 + 127 * Math.sin(t * 2 + i / 14); break;                      // onda
        case 1: v = (i % 8 < 3) ? 255 * Math.abs(Math.sin(t + Math.floor(i / 8) / 5)) : 0; break; // RGB por aparato
        case 2: v = Math.max(0, 255 - Math.abs(((t * 90) % 600) - i) * 6); break;      // barrido
        default: v = (Math.floor(i / 32) + Math.floor(t * 2)) % 2 ? 220 : 30;          // bloques
      }
      out[i] = v;
    }
    return out;
  }

  function state() {
    const routed = new Set();
    const devices = S.devs.map(d => {
      const run = running(d);
      if (run) d.universes.forEach(u => routed.add(u));
      return {
        id: d.id, name: d.name, port: d.port, serial: d.serial, kind: d.kind, kindLabel: d.kindLabel,
        kindMode: d.kindMode, detail: d.detail, present: !d.absent, running: run, enabled: d.enabled,
        test: d.test && run, error: '',
        outputs: d.universes.map(u => ({ universe: u, fps: run ? (d.kind === 'opendmx' || live(u) || d.test ? jitter(d.hz) : 1) : 0, signal: live(u) })),
      };
    });
    return {
      version: '1.0.0', listening: true, listenError: '', scanError: '',
      listenIP: S.listenIP, sourceIP: S.sourceIP, firstUniverse: S.firstUniverse, localIPs: S.localIPs,
      sources: S.senders.map(ip => ({ ip, active: true })),
      universes: accepted() ? S.incoming.map(u => ({ universe: u, pps: jitter(40), active: true, source: S.senders[0], routed: routed.has(u) })) : [],
      devices, otherPorts: S.others.map(p => ({ ...p })), log: [...log],
    };
  }

  const fail = error => ({ status: 400, body: { error } });
  function handle(path, b) {
    const d = S.devs.find(x => x.id === b.id);
    switch (path) {
      case '/api/network': {
        const ip4 = /^(\d{1,3}\.){3}\d{1,3}$/;
        if (b.sourceIP && !ip4.test(b.sourceIP.trim())) return fail('IP de origen inválida');
        if (!(b.firstUniverse >= 0 && b.firstUniverse <= 32767)) return fail('universo inicial fuera de rango (0-32767)');
        if (b.listenIP !== S.listenIP) say(`Art-Net: escuchando en ${b.listenIP}:6454`);
        Object.assign(S, { listenIP: b.listenIP, sourceIP: (b.sourceIP || '').trim(), firstUniverse: b.firstUniverse });
        return {};
      }
      case '/api/device':
        if (!d) return fail('interfaz desconocida');
        if (b.universes) {
          if (b.universes.some(u => !(u >= 0 && u <= 32767))) return fail('universo fuera de rango (0-32767)');
          d.universes = b.universes;
        }
        if (typeof b.enabled === 'boolean') d.enabled = b.enabled;
        if (typeof b.test === 'boolean') d.test = b.test;
        if (typeof b.kind === 'string') d.kindMode = b.kind;
        return {};
      case '/api/forget':
        S.devs = S.devs.filter(x => x !== d);
        return {};
      case '/api/addport': {
        const p = S.others.find(x => x.name === b.port);
        if (!p) return fail('puerto no encontrado');
        S.others = S.others.filter(x => x !== p);
        const used = new Set(S.devs.flatMap(x => x.universes));
        let u = S.firstUniverse; while (used.has(u)) u++;
        S.devs.push({ id: 'port:' + p.name, name: p.desc, port: p.name, serial: '', kind: 'opendmx', kindLabel: 'Open DMX', detail: '', universes: [u], hz: 36, enabled: true, test: false, kindMode: '' });
        say(`Interfaz detectada: ${p.desc} en ${p.name}`);
        say(`${p.name} (${p.desc}) lista: Open DMX → universos [${u}]`);
        return {};
      }
      case '/api/quit':
        return {};
    }
    return { status: 404, body: { error: 'no existe' } };
  }

  const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
  window.fetch = async (url, opts = {}) => {
    const u = new URL(url, location.href);
    if (u.pathname.endsWith('/api/state')) return json(state());
    if (u.pathname.endsWith('/api/levels')) {
      const n = parseInt(u.searchParams.get('u')) || 0;
      return json({ universe: n, levels: btoa(String.fromCharCode(...levels(n))) });
    }
    const r = handle(u.pathname.slice(u.pathname.lastIndexOf('/api/')), JSON.parse(opts.body || '{}'));
    return json(r.body || { ok: true }, r.status || 200);
  };

  document.addEventListener('DOMContentLoaded', () => {
    const bar = document.createElement('div');
    bar.style.cssText = 'background:#ffb000;color:#1a1300;text-align:center;padding:8px 14px;font-weight:600;font-size:13px';
    bar.innerHTML = 'Demo en línea con datos simulados: aquí no hay interfaces ni Art-Net reales. ' +
      '<a href="https://github.com/mariance95/lj-buddy" style="color:inherit">Código y descargas en GitHub</a>';
    bar.style.borderRadius = "8px";
    document.querySelector("main").prepend(bar);
  });
})();
