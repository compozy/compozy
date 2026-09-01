/**
 * A stand-in for `document.fonts` — the boundary the font-readiness module
 * drives. It records which specs were asked for and lets a test decide when
 * (and whether) each load lands, so assertions cover real sequencing: paint
 * gated within budget, residency flipping only on settle.
 */

export interface FakeFontFaceSet {
  readonly loadCalls: Array<{ font: string; text: string | undefined }>;
  /** Specs `check` reports as resident right now. */
  readonly resident: Set<string>;
  check(font: string, text?: string): boolean;
  load(font: string, text?: string): Promise<FontFace[]>;
  /** Lands every pending load: marks its spec resident and resolves it. */
  settleLoads(): void;
  /** Fails every pending load without making anything resident. */
  failLoads(): void;
}

export function createFakeFontFaceSet(residentSpecs: string[] = []): FakeFontFaceSet {
  const resident = new Set<string>(residentSpecs);
  const pending: Array<{ spec: string; resolve: () => void; reject: (cause: Error) => void }> = [];
  const set: FakeFontFaceSet = {
    loadCalls: [],
    resident,
    check: font => resident.has(font),
    load: (font, text) => {
      set.loadCalls.push({ font, text });
      return new Promise<FontFace[]>((resolve, reject) => {
        pending.push({
          spec: font,
          resolve: () => {
            resident.add(font);
            resolve([]);
          },
          reject,
        });
      });
    },
    settleLoads: () => {
      for (const entry of pending.splice(0)) entry.resolve();
    },
    failLoads: () => {
      for (const entry of pending.splice(0)) entry.reject(new Error("font load failed"));
    },
  };
  return set;
}

/**
 * Installs the fake as `document.fonts` and returns the undo. jsdom ships no
 * `FontFaceSet`, so production code sees exactly what a browser would offer.
 */
export function installFakeFontFaceSet(fonts: FakeFontFaceSet): () => void {
  Object.defineProperty(document, "fonts", {
    configurable: true,
    value: fonts as unknown as FontFaceSet,
  });
  return () => {
    Reflect.deleteProperty(document, "fonts");
  };
}
