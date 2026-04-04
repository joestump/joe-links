// Generates a 440x280 Chrome Web Store promotional tile.
// Purple gradient background with chain-link icon and "joe-links" text.
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
function encodePNG(w, h, rgb) {
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
  ihdr[8] = 8; ihdr[9] = 2; // 8-bit RGB

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

// ---- 5x7 bitmap font for "joe-links" ----
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
  const charW = 7 * scale + Math.ceil(scale * 0.5); // character width + spacing
  for (let i = 0; i < text.length; i++) {
    drawChar(pixels, w, h, text[i], ox + i * charW, oy, scale, r, g, b);
  }
  return text.length * charW;
}

// ---- Generate promo tile ----
const W = 440, H = 280;
const pixels = Buffer.alloc(W * H * 3);

// Gradient background: top #a855f7 → bottom #7c3aed
for (let y = 0; y < H; y++) {
  const t = y / (H - 1);
  const r = Math.round(168 + (124 - 168) * t); // 0xa8 → 0x7c
  const g = Math.round(85  + (58  - 85)  * t); // 0x55 → 0x3a
  const b = Math.round(247 + (237 - 247) * t); // 0xf7 → 0xed
  for (let x = 0; x < W; x++) {
    setPixel(pixels, W, H, x, y, r, g, b);
  }
}

// Draw chain-link icon (centered, scaled from 24x24 SVG viewbox)
const iconSize = 80;
const iconX = (W - iconSize) / 2;
const iconY = 50;
const s  = iconSize / 24;
const sw = Math.max(1.0, 1.5 * s);

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

// "joe-links" text
const textScale = 5;
const textStr = 'joe-links';
const textW = textStr.length * (7 * textScale + Math.ceil(textScale * 0.5));
const textX = Math.round((W - textW) / 2);
const textY = 170;
drawText(pixels, W, H, textStr, textX, textY, textScale, 255, 255, 255);

// Tagline: "go/links made easy" — smaller scale
const tagScale = 2;
const tagStr = 'go/links made easy';
const tagW = tagStr.length * (7 * tagScale + Math.ceil(tagScale * 0.5));
const tagX = Math.round((W - tagW) / 2);
const tagY = 225;
drawText(pixels, W, H, tagStr, tagX, tagY, tagScale, 230, 210, 255);

const png = encodePNG(W, H, pixels);
const outPath = path.join(__dirname, '..', '..', '..', 'joe-links-promo-440x280.png');
fs.writeFileSync(outPath, png);
console.log(`Created ${outPath} (${png.length} bytes)`);
