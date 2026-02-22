// Generates chain-link PNG icons without npm dependencies.
// Draws the Heroicons "link" path (SVG viewBox 0 0 24 24) on a purple background.
'use strict';
const fs   = require('fs');
const path = require('path');
const zlib = require('zlib');

// ---- CRC32 ----
function crc32(buf) {
  const table = new Int32Array(256);
  for (let i = 0; i < 256; i++) {
    let c = i;
    for (let j = 0; j < 8; j++) c = (c & 1) ? (0xEDB88320 ^ (c >>> 1)) : (c >>> 1);
    table[i] = c;
  }
  let crc = -1;
  for (let i = 0; i < buf.length; i++) crc = (crc >>> 8) ^ table[(crc ^ buf[i]) & 0xFF];
  return (crc ^ -1) >>> 0;
}

// ---- PNG encoder (RGB) ----
function encodePNG(size, rgb) {
  const rowSize = 1 + size * 3;
  const raw = Buffer.alloc(rowSize * size);
  for (let y = 0; y < size; y++) {
    raw[y * rowSize] = 0; // filter=None
    rgb.copy(raw, y * rowSize + 1, y * size * 3, (y + 1) * size * 3);
  }
  const compressed = zlib.deflateSync(raw, { level: 9 });

  function chunk(type, data) {
    const t = Buffer.from(type, 'ascii');
    const crc = crc32(Buffer.concat([t, data]));
    const out = Buffer.alloc(4 + 4 + data.length + 4);
    out.writeUInt32BE(data.length, 0);
    t.copy(out, 4);
    data.copy(out, 8);
    out.writeUInt32BE(crc, 8 + data.length);
    return out;
  }

  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(size, 0); ihdr.writeUInt32BE(size, 4);
  ihdr[8] = 8; ihdr[9] = 2; // 8-bit RGB

  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', ihdr),
    chunk('IDAT', compressed),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

// ---- Drawing ----
const BG_R = 0xC0, BG_G = 0x84, BG_B = 0xFC; // #c084fc

function drawIcon(size) {
  const pixels = Buffer.alloc(size * size * 3);
  for (let i = 0; i < size * size; i++) {
    pixels[i * 3] = BG_R; pixels[i * 3 + 1] = BG_G; pixels[i * 3 + 2] = BG_B;
  }

  const s  = size / 24;                       // scale from 24×24 SVG viewbox
  const sw = Math.max(1.0, 1.25 * s);         // stroke half-width

  // Alpha-blend white onto pixel at integer (px, py)
  function paint(px, py, alpha) {
    const x = Math.round(px), y = Math.round(py);
    if (x < 0 || x >= size || y < 0 || y >= size) return;
    const a = Math.min(1, Math.max(0, alpha));
    const i = (y * size + x) * 3;
    pixels[i]     = Math.round(pixels[i]     + (255 - pixels[i])     * a);
    pixels[i + 1] = Math.round(pixels[i + 1] + (255 - pixels[i + 1]) * a);
    pixels[i + 2] = Math.round(pixels[i + 2] + (255 - pixels[i + 2]) * a);
  }

  // Draw an anti-aliased thick point at floating-point (x, y) with half-radius r
  function dot(x, y, r) {
    const ir = Math.ceil(r) + 1;
    for (let dy = -ir; dy <= ir; dy++) {
      for (let dx = -ir; dx <= ir; dx++) {
        const d = Math.sqrt(dx * dx + dy * dy);
        paint(x + dx, y + dy, Math.min(1, r - d + 0.5));
      }
    }
  }

  // Stroke an arc: center (cx,cy) radius r, from startDeg to endDeg
  // Direction is determined by sign of (endDeg - startDeg).
  function strokeArc(cx, cy, r, startDeg, endDeg) {
    const start = startDeg * Math.PI / 180;
    const end   = endDeg   * Math.PI / 180;
    const steps = Math.max(4, Math.ceil(r * Math.abs(end - start) * 4));
    for (let i = 0; i <= steps; i++) {
      const a = start + (end - start) * i / steps;
      dot(cx + r * Math.cos(a), cy + r * Math.sin(a), sw);
    }
  }

  // Stroke a line from (x1,y1) to (x2,y2)
  function strokeLine(x1, y1, x2, y2) {
    const dx = x2 - x1, dy = y2 - y1;
    const steps = Math.max(2, Math.ceil(Math.sqrt(dx * dx + dy * dy) * 4));
    for (let i = 0; i <= steps; i++) {
      dot(x1 + dx * i / steps, y1 + dy * i / steps, sw);
    }
  }

  // Shorthand: scale from SVG-space to pixel-space
  function sc(v) { return v * s; }

  // === Heroicons "link" path, decomposed into arcs and lines ===
  //
  // SVG: M13.828 10.172 a4 4 0 00-5.656 0 l-4 4 a4 4 0 105.656 5.656 l1.102-1.101
  //      m-.758-4.899   a4 4 0 005.656 0 l4-4  a4 4 0 00-5.656-5.656 l-1.1 1.1
  //
  // Two open sub-paths; each sub-path traces one chain ring.
  // Key arc centers (computed from SVG arc endpoints + radius):
  //   Arc 1 center: (11, 13)  — top curve of lower-left ring
  //   Arc 2 center: (7, 17)   — bottom curve of lower-left ring
  //   Arc 3 center: (13, 11)  — bottom curve of upper-right ring
  //   Arc 4 center: (17, 7)   — top curve of upper-right ring

  // --- Upper-right ring (drawn first = visually behind) ---
  strokeArc(sc(13), sc(11), sc(4), 135,  45);        // 90° arc CCW (decreasing angle)
  strokeLine(sc(15.828), sc(13.828), sc(19.828), sc(9.828));
  strokeArc(sc(17), sc(7),  sc(4),  45, 225);        // 180° arc CW (increasing angle)
  strokeLine(sc(14.172), sc(4.172), sc(13.072), sc(5.272));

  // --- Lower-left ring (drawn second = visually in front) ---
  strokeArc(sc(11), sc(13), sc(4), -45, -135);       // 90° arc CCW (decreasing angle)
  strokeLine(sc(8.172), sc(10.172), sc(4.172), sc(14.172));
  strokeArc(sc(7),  sc(17), sc(4), 225,  405);       // 180° arc CW (225°→405°=45°)
  strokeLine(sc(9.828), sc(19.828), sc(10.930), sc(18.727));

  return encodePNG(size, pixels);
}

// ---- Generate ----
const sizes  = [16, 48, 128];
const outDir = path.dirname(__filename);

sizes.forEach(size => {
  const png     = drawIcon(size);
  const outPath = path.join(outDir, `icon-${size}.png`);
  fs.writeFileSync(outPath, png);
  console.log(`Created ${outPath} (${png.length} bytes)`);
});
