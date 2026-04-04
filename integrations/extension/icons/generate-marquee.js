// Generates a 1400x560 Chrome Web Store marquee promotional tile.
// Purple gradient background with chain-link icon and "joe-links" text.
// 24-bit PNG (no alpha) as required by Chrome Web Store.
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

// ---- PNG encoder (RGB, no alpha) ----
function encodePNG(w, h, rgb) {
  // Use Sub filter for better compression on gradient backgrounds
  const rowSize = 1 + w * 3;
  const raw = Buffer.alloc(rowSize * h);
  for (let y = 0; y < h; y++) {
    raw[y * rowSize] = 0; // filter=None
    rgb.copy(raw, y * rowSize + 1, y * w * 3, (y + 1) * w * 3);
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
  ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4);
  ihdr[8] = 8; ihdr[9] = 2; // 8-bit RGB (no alpha)

  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', ihdr),
    chunk('IDAT', compressed),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

// ---- Drawing helpers ----
function setPixel(pixels, w, h, x, y, r, g, b) {
  if (x < 0 || x >= w || y < 0 || y >= h) return;
  const i = (y * w + x) * 3;
  pixels[i] = r; pixels[i + 1] = g; pixels[i + 2] = b;
}

function blend(pixels, w, h, x, y, r, g, b, a) {
  x = Math.round(x); y = Math.round(y);
  if (x < 0 || x >= w || y < 0 || y >= h) return;
  const alpha = Math.min(1, Math.max(0, a));
  const i = (y * w + x) * 3;
  pixels[i]     = Math.round(pixels[i]     + (r - pixels[i])     * alpha);
  pixels[i + 1] = Math.round(pixels[i + 1] + (g - pixels[i + 1]) * alpha);
  pixels[i + 2] = Math.round(pixels[i + 2] + (b - pixels[i + 2]) * alpha);
}

function dot(pixels, w, h, cx, cy, radius, r, g, b) {
  const ir = Math.ceil(radius) + 1;
  for (let dy = -ir; dy <= ir; dy++) {
    for (let dx = -ir; dx <= ir; dx++) {
      const d = Math.sqrt(dx * dx + dy * dy);
      blend(pixels, w, h, cx + dx, cy + dy, r, g, b, Math.min(1, radius - d + 0.5));
    }
  }
}

function strokeArc(pixels, w, h, cx, cy, radius, startDeg, endDeg, sw, r, g, b) {
  const start = startDeg * Math.PI / 180;
  const end   = endDeg   * Math.PI / 180;
  const steps = Math.max(4, Math.ceil(radius * Math.abs(end - start) * 4));
  for (let i = 0; i <= steps; i++) {
    const a = start + (end - start) * i / steps;
    dot(pixels, w, h, cx + radius * Math.cos(a), cy + radius * Math.sin(a), sw, r, g, b);
  }
}

function strokeLine(pixels, w, h, x1, y1, x2, y2, sw, r, g, b) {
  const dx = x2 - x1, dy = y2 - y1;
  const steps = Math.max(2, Math.ceil(Math.sqrt(dx * dx + dy * dy) * 4));
  for (let i = 0; i <= steps; i++) {
    dot(pixels, w, h, x1 + dx * i / steps, y1 + dy * i / steps, sw, r, g, b);
  }
}

// ---- 5x7 bitmap font ----
const FONT = {
  'j': [0x04,0x04,0x04,0x04,0x44,0x44,0x38],
  'o': [0x00,0x38,0x44,0x44,0x44,0x44,0x38],
  'e': [0x00,0x38,0x44,0x7C,0x40,0x44,0x38],
  '-': [0x00,0x00,0x00,0x7C,0x00,0x00,0x00],
  'l': [0x30,0x10,0x10,0x10,0x10,0x10,0x38],
  'i': [0x10,0x00,0x30,0x10,0x10,0x10,0x38],
  'n': [0x00,0x58,0x64,0x44,0x44,0x44,0x44],
  'k': [0x40,0x44,0x48,0x50,0x70,0x48,0x44],
  's': [0x00,0x38,0x40,0x38,0x04,0x04,0x38],
  'g': [0x00,0x3C,0x44,0x44,0x3C,0x04,0x38],
  'a': [0x00,0x38,0x44,0x44,0x7C,0x44,0x44],
  'd': [0x04,0x04,0x3C,0x44,0x44,0x44,0x3C],
  'y': [0x00,0x44,0x44,0x3C,0x04,0x04,0x38],
  'm': [0x00,0x68,0x54,0x54,0x44,0x44,0x44],
  '/': [0x04,0x04,0x08,0x10,0x20,0x40,0x40],
  ' ': [0x00,0x00,0x00,0x00,0x00,0x00,0x00],
  'f': [0x1C,0x20,0x20,0x78,0x20,0x20,0x20],
  'h': [0x40,0x40,0x58,0x64,0x44,0x44,0x44],
  't': [0x20,0x20,0x78,0x20,0x20,0x24,0x18],
  'r': [0x00,0x58,0x64,0x40,0x40,0x40,0x40],
  'p': [0x00,0x78,0x44,0x44,0x78,0x40,0x40],
  'u': [0x00,0x44,0x44,0x44,0x44,0x4C,0x34],
  'b': [0x40,0x40,0x78,0x44,0x44,0x44,0x78],
  'w': [0x00,0x44,0x44,0x44,0x54,0x54,0x28],
  'c': [0x00,0x38,0x44,0x40,0x40,0x44,0x38],
};

function drawChar(pixels, w, h, ch, ox, oy, scale, r, g, b) {
  const glyph = FONT[ch];
  if (!glyph) return;
  for (let row = 0; row < 7; row++) {
    for (let col = 0; col < 7; col++) {
      if (glyph[row] & (0x80 >> col)) {
        for (let sy = 0; sy < scale; sy++) {
          for (let sx = 0; sx < scale; sx++) {
            const px = ox + col * scale + sx;
            const py = oy + row * scale + sy;
            if (px >= 0 && px < w && py >= 0 && py < h) {
              setPixel(pixels, w, h, px, py, r, g, b);
            }
          }
        }
      }
    }
  }
}

function drawText(pixels, w, h, text, ox, oy, scale, r, g, b) {
  const charW = 7 * scale + Math.ceil(scale * 0.5);
  for (let i = 0; i < text.length; i++) {
    drawChar(pixels, w, h, text[i], ox + i * charW, oy, scale, r, g, b);
  }
  return text.length * charW;
}

function textWidth(text, scale) {
  return text.length * (7 * scale + Math.ceil(scale * 0.5));
}

// ---- Generate marquee tile ----
const W = 1400, H = 560;
const pixels = Buffer.alloc(W * H * 3);

// Gradient background: top #a855f7 → bottom #7c3aed
for (let y = 0; y < H; y++) {
  const t = y / (H - 1);
  const r = Math.round(168 + (124 - 168) * t);
  const g = Math.round(85  + (58  - 85)  * t);
  const b = Math.round(247 + (237 - 247) * t);
  for (let x = 0; x < W; x++) {
    setPixel(pixels, W, H, x, y, r, g, b);
  }
}

// Draw chain-link icon (left side)
const iconSize = 160;
const iconX = 280;
const iconY = (H - iconSize) / 2;
const s  = iconSize / 24;
const sw = Math.max(1.0, 2.0 * s);

function sc(v) { return v * s; }

// Upper-right ring
strokeArc(pixels, W, H, iconX + sc(13), iconY + sc(11), sc(4), 135, 45, sw, 255, 255, 255);
strokeLine(pixels, W, H, iconX + sc(15.828), iconY + sc(13.828), iconX + sc(19.828), iconY + sc(9.828), sw, 255, 255, 255);
strokeArc(pixels, W, H, iconX + sc(17), iconY + sc(7), sc(4), 45, 225, sw, 255, 255, 255);
strokeLine(pixels, W, H, iconX + sc(14.172), iconY + sc(4.172), iconX + sc(13.072), iconY + sc(5.272), sw, 255, 255, 255);

// Lower-left ring
strokeArc(pixels, W, H, iconX + sc(11), iconY + sc(13), sc(4), -45, -135, sw, 255, 255, 255);
strokeLine(pixels, W, H, iconX + sc(8.172), iconY + sc(10.172), iconX + sc(4.172), iconY + sc(14.172), sw, 255, 255, 255);
strokeArc(pixels, W, H, iconX + sc(7), iconY + sc(17), sc(4), 225, 405, sw, 255, 255, 255);
strokeLine(pixels, W, H, iconX + sc(9.828), iconY + sc(19.828), iconX + sc(10.930), iconY + sc(18.727), sw, 255, 255, 255);

// "joe-links" — large text, right of icon
const titleScale = 10;
const titleStr = 'joe-links';
const titleW = textWidth(titleStr, titleScale);
const titleX = iconX + iconSize + 80;
const titleY = Math.round(H / 2 - (7 * titleScale) / 2 - 30);
drawText(pixels, W, H, titleStr, titleX, titleY, titleScale, 255, 255, 255);

// "go/links made easy" tagline below
const tagScale = 4;
const tagStr = 'go/links made easy';
const tagX = titleX;
const tagY = titleY + 7 * titleScale + 25;
drawText(pixels, W, H, tagStr, tagX, tagY, tagScale, 230, 210, 255);

const png = encodePNG(W, H, pixels);
const outPath = path.join(__dirname, '..', '..', '..', 'joe-links-marquee-1400x560.png');
fs.writeFileSync(outPath, png);
console.log(`Created ${outPath} (${png.length} bytes)`);
