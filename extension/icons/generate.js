// Generates solid-color PNG icons without any npm dependencies.
// Uses raw PNG binary format (IHDR + IDAT + IEND chunks).
const fs = require('fs');
const path = require('path');
const zlib = require('zlib');

function crc32(buf) {
  let table = new Int32Array(256);
  for (let i = 0; i < 256; i++) {
    let c = i;
    for (let j = 0; j < 8; j++) {
      c = (c & 1) ? (0xEDB88320 ^ (c >>> 1)) : (c >>> 1);
    }
    table[i] = c;
  }
  let crc = -1;
  for (let i = 0; i < buf.length; i++) {
    crc = (crc >>> 8) ^ table[(crc ^ buf[i]) & 0xFF];
  }
  return (crc ^ -1) >>> 0;
}

function createPNG(size, r, g, b) {
  // PNG signature
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);

  // IHDR chunk
  const ihdrData = Buffer.alloc(13);
  ihdrData.writeUInt32BE(size, 0);   // width
  ihdrData.writeUInt32BE(size, 4);   // height
  ihdrData[8] = 8;                    // bit depth
  ihdrData[9] = 2;                    // color type (RGB)
  ihdrData[10] = 0;                   // compression
  ihdrData[11] = 0;                   // filter
  ihdrData[12] = 0;                   // interlace

  const ihdrType = Buffer.from('IHDR');
  const ihdrCRC = crc32(Buffer.concat([ihdrType, ihdrData]));
  const ihdr = Buffer.alloc(4 + 4 + 13 + 4);
  ihdr.writeUInt32BE(13, 0);
  ihdrType.copy(ihdr, 4);
  ihdrData.copy(ihdr, 8);
  ihdr.writeUInt32BE(ihdrCRC, 21);

  // IDAT chunk - raw pixel data
  // Each row: filter byte (0) + RGB pixels
  const rowSize = 1 + size * 3;
  const rawData = Buffer.alloc(rowSize * size);
  for (let y = 0; y < size; y++) {
    const offset = y * rowSize;
    rawData[offset] = 0; // no filter
    for (let x = 0; x < size; x++) {
      const px = offset + 1 + x * 3;
      rawData[px] = r;
      rawData[px + 1] = g;
      rawData[px + 2] = b;
    }
  }

  const compressed = zlib.deflateSync(rawData);
  const idatType = Buffer.from('IDAT');
  const idatCRC = crc32(Buffer.concat([idatType, compressed]));
  const idat = Buffer.alloc(4 + 4 + compressed.length + 4);
  idat.writeUInt32BE(compressed.length, 0);
  idatType.copy(idat, 4);
  compressed.copy(idat, 8);
  idat.writeUInt32BE(idatCRC, 8 + compressed.length);

  // IEND chunk
  const iendType = Buffer.from('IEND');
  const iendCRC = crc32(iendType);
  const iend = Buffer.alloc(12);
  iend.writeUInt32BE(0, 0);
  iendType.copy(iend, 4);
  iend.writeUInt32BE(iendCRC, 8);

  return Buffer.concat([signature, ihdr, idat, iend]);
}

const sizes = [16, 48, 128];
const outDir = path.dirname(__filename);

// Solid blue #3B82F6
sizes.forEach(size => {
  const png = createPNG(size, 0x3B, 0x82, 0xF6);
  const outPath = path.join(outDir, `icon-${size}.png`);
  fs.writeFileSync(outPath, png);
  console.log(`Created ${outPath} (${png.length} bytes)`);
});
