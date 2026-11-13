/** A parsed delimited-text table: the first record is the header row. */
export interface DelimitedTable {
  header: string[];
  rows: string[][];
  /** Widest record, so short rows pad to a rectangle. */
  columnCount: number;
}

/** Comma for `.csv`, tab for `.tsv`/`.tab`; any other path parses as comma-separated. */
export function delimiterForPath(path: string): "," | "\t" {
  return /\.(tsv|tab)$/i.test(path) ? "\t" : ",";
}

/**
 * RFC 4180 records: quoted fields may hold the delimiter, doubled quotes and line breaks;
 * CRLF and LF both end a record. A trailing newline adds no empty record, and a truncated
 * read that stops inside a quoted field keeps what it read.
 */
export function parseDelimited(text: string, delimiter: string): DelimitedTable {
  const records: string[][] = [];
  let record: string[] = [];
  let field = "";
  let quoted = false;
  let index = 0;
  const source = text.charCodeAt(0) === 0xfeff ? text.slice(1) : text;
  const endRecord = () => {
    record.push(field);
    records.push(record);
    record = [];
    field = "";
  };
  while (index < source.length) {
    const char = source[index];
    if (quoted) {
      if (char === '"') {
        if (source[index + 1] === '"') {
          field += '"';
          index += 2;
          continue;
        }
        quoted = false;
      } else {
        field += char;
      }
      index += 1;
      continue;
    }
    if (char === '"' && field === "") {
      quoted = true;
    } else if (char === delimiter) {
      record.push(field);
      field = "";
    } else if (char === "\n" || char === "\r") {
      endRecord();
      if (char === "\r" && source[index + 1] === "\n") index += 1;
    } else {
      field += char;
    }
    index += 1;
  }
  if (field !== "" || record.length > 0) endRecord();
  const columnCount = records.reduce((widest, row) => Math.max(widest, row.length), 0);
  const [header = [], ...rows] = records;
  return { header, rows, columnCount };
}
