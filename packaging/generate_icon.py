"""Render the app's own simple window icon as a build-time PNG (stdlib only)."""
from pathlib import Path
import struct
import zlib

size = 1024
background = bytes((45, 49, 43, 255))
foreground = bytes((215, 226, 207, 255))
transparent = bytes(4)
rows = bytearray()
for y in range(size):
    py = y * 64 // size
    rows.append(0)
    for x in range(size):
        px = x * 64 // size
        color = background if 4 <= px < 60 and 4 <= py < 60 else transparent
        if (15 <= px <= 48 and py in (16, 17, 27, 28, 47, 48)) or (16 <= py <= 48 and px in (15, 16, 47, 48)) or (26 <= px <= 27 and 28 <= py <= 47):
            color = foreground
        rows.extend(color)

def chunk(kind, data):
    return struct.pack('>I', len(data)) + kind + data + struct.pack('>I', zlib.crc32(kind + data) & 0xffffffff)

png = b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', size, size, 8, 6, 0, 0, 0))
png += chunk(b'IDAT', zlib.compress(rows, 9)) + chunk(b'IEND', b'')
Path(__file__).with_name('icon.png').write_bytes(png)
