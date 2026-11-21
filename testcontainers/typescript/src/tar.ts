// A small reader for the tar archives that the Docker archive API returns.
//
// GET /containers/{id}/archive returns a tar stream with one entry for a
// file path. This reader returns the content of the first regular file in
// the archive. It knows the ustar, PAX ("x", "g") and GNU long name ("L",
// "K") header types, so the library does not need a tar dependency.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

const BLOCK = 512;

/** Read all bytes of a stream into one buffer. */
export async function readStream(stream: NodeJS.ReadableStream): Promise<Buffer> {
  const chunks: Buffer[] = [];
  for await (const chunk of stream) {
    chunks.push(typeof chunk === "string" ? Buffer.from(chunk) : Buffer.from(chunk));
  }
  return Buffer.concat(chunks);
}

/** Return the content of the first regular file in a tar archive. */
export function firstFileFromTar(archive: Buffer): Buffer {
  let offset = 0;
  while (offset + BLOCK <= archive.length) {
    const header = archive.subarray(offset, offset + BLOCK);
    // Two zero blocks end the archive. One zero block is enough to stop.
    if (header.every((b) => b === 0)) break;

    const size = parseOctal(header.subarray(124, 136));
    const type = String.fromCharCode(header[156] ?? 0);
    const dataStart = offset + BLOCK;
    const dataEnd = dataStart + size;
    if (dataEnd > archive.length) {
      throw new Error("The tar archive from the container is truncated");
    }
    // "0" and NUL are regular files. "7" is a contiguous file (also regular).
    if (type === "0" || type === "\0" || type === "7") {
      return Buffer.from(archive.subarray(dataStart, dataEnd));
    }
    // Skip all other entries: directories, links, and extension headers.
    offset = dataStart + Math.ceil(size / BLOCK) * BLOCK;
  }
  throw new Error("The tar archive from the container has no regular file");
}

function parseOctal(field: Buffer): number {
  // GNU base-256 encoding: the high bit of the first byte is set.
  if (((field[0] ?? 0) & 0x80) !== 0) {
    let value = 0;
    for (let i = 1; i < field.length; i++) value = value * 256 + (field[i] ?? 0);
    return value;
  }
  const text = field.toString("ascii").replace(/\0.*$/s, "").trim();
  return text === "" ? 0 : Number.parseInt(text, 8);
}
