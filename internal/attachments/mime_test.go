package attachments

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"slices"
	"strings"
	"testing"
)

func TestSniffMIME(t *testing.T) {
	t.Parallel()

	pngBytes := encodeTestPNG(t, 3, 2)
	jpegBytes := encodeTestJPEG(t, 4, 5)

	t.Run("Should detect binary formats", func(t *testing.T) {
		t.Parallel()
		testSniffMIMECases(t, []sniffMIMECase{
			{name: "Should detect PNG from magic bytes", data: pngBytes, filename: "note.txt", wantMIME: MIMEImagePNG, wantKind: KindImage, wantW: 3, wantH: 2},
			{name: "Should detect JPEG when the extension claims PNG", data: jpegBytes, filename: "shot.png", wantMIME: MIMEImageJPEG, wantKind: KindImage, wantW: 4, wantH: 5},
			{name: "Should detect extended WebP dimensions", data: testWebPVP8X(6, 7), filename: "canvas.bin", wantMIME: MIMEImageWebP, wantKind: KindImage, wantW: 6, wantH: 7},
			{name: "Should detect lossy WebP dimensions", data: testWebPVP8(8, 9), filename: "canvas.bin", wantMIME: MIMEImageWebP, wantKind: KindImage, wantW: 8, wantH: 9},
			{name: "Should detect lossless WebP dimensions", data: testWebPVP8L(10, 11), filename: "canvas.bin", wantMIME: MIMEImageWebP, wantKind: KindImage, wantW: 10, wantH: 11},
			{name: "Should detect PDF from the percent-PDF signature", data: []byte("%PDF-1.4\n%"), filename: "spec.md", wantMIME: MIMEApplicationPDF, wantKind: KindFile},
		})
	})

	t.Run("Should classify text", func(t *testing.T) {
		t.Parallel()
		testSniffMIMECases(t, []sniffMIMECase{
			{name: "Should treat valid UTF-8 with a markdown extension as markdown", data: []byte("# notes\n"), filename: "notes.md", wantMIME: MIMETextMarkdown, wantKind: KindFile},
			{name: "Should treat valid UTF-8 without a markdown extension as plain text", data: []byte("hello"), filename: "notes.txt", wantMIME: MIMETextPlain, wantKind: KindFile},
		})
	})

	t.Run("Should reject unsupported bytes", func(t *testing.T) {
		t.Parallel()
		testSniffMIMECases(t, []sniffMIMECase{
			{name: "Should reject GIF bytes even when named as PNG", data: []byte("GIF89a"), filename: "anim.png", wantErr: true},
			{name: "Should reject ZIP bytes", data: []byte("PK\x03\x04archive"), filename: "notes.md", wantErr: true},
			{name: "Should reject non-WebP RIFF bytes", data: []byte("RIFF\x04\x00\x00\x00WAVE"), filename: "audio.wav", wantErr: true},
			{name: "Should reject SVG markup", data: []byte("<svg></svg>"), filename: "drawing.txt", wantErr: true},
			{name: "Should reject XML-wrapped SVG markup", data: []byte("<?xml version=\"1.0\"?><svg></svg>"), filename: "drawing.txt", wantErr: true},
			{name: "Should reject binary that is not valid UTF-8", data: []byte{0x00, 0xff, 0xfe, 0x01}, filename: "notes.txt", wantErr: true},
			{name: "Should reject UTF-8 control bytes disguised as markdown", data: []byte{'n', 'o', 't', 'e', 0x01}, filename: "notes.md", wantErr: true},
		})
	})
}

func TestDefaultAllowedMIMEs(t *testing.T) {
	t.Parallel()

	t.Run("Should return an independent canonical list", func(t *testing.T) {
		t.Parallel()

		first := DefaultAllowedMIMEs()
		first[0] = "application/test"
		got := DefaultAllowedMIMEs()
		want := []string{
			MIMEImagePNG,
			MIMEImageJPEG,
			MIMEImageWebP,
			MIMEApplicationPDF,
			MIMETextMarkdown,
			MIMETextPlain,
		}
		if !slices.Equal(got, want) {
			t.Fatalf("DefaultAllowedMIMEs() = %#v, want %#v", got, want)
		}
	})
}

func testSniffMIMECases(t *testing.T, cases []sniffMIMECase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mime, kind, width, height, err := SniffMIME(tc.data, tc.filename)
			if tc.wantErr {
				var unsupported *UnsupportedMIMEError
				if !errors.As(err, &unsupported) || !errors.Is(err, ErrUnsupportedMIME) {
					t.Fatalf("SniffMIME() error = %v, want unsupported MIME", err)
				}
				if !strings.Contains(unsupported.Error(), MIMEImagePNG) {
					t.Fatalf("SniffMIME() error = %q, want allowed types named", unsupported.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("SniffMIME() error = %v", err)
			}
			if mime != tc.wantMIME || kind != tc.wantKind || width != tc.wantW || height != tc.wantH {
				t.Fatalf(
					"SniffMIME() = %q %q %dx%d, want %q %q %dx%d",
					mime,
					kind,
					width,
					height,
					tc.wantMIME,
					tc.wantKind,
					tc.wantW,
					tc.wantH,
				)
			}
		})
	}
}

type sniffMIMECase struct {
	name     string
	data     []byte
	filename string
	wantMIME string
	wantKind string
	wantW    int
	wantH    int
	wantErr  bool
}

func encodeTestPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func encodeTestJPEG(t *testing.T, width int, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func testWebPVP8X(width int, height int) []byte {
	payload := make([]byte, 10)
	w := uint32(width - 1)
	h := uint32(height - 1)
	payload[4] = byte(w)
	payload[5] = byte(w >> 8)
	payload[6] = byte(w >> 16)
	payload[7] = byte(h)
	payload[8] = byte(h >> 8)
	payload[9] = byte(h >> 16)

	out := make([]byte, 30)
	copy(out[0:4], "RIFF")
	out[4] = 22
	copy(out[8:12], "WEBP")
	copy(out[12:16], "VP8X")
	out[16] = 10
	copy(out[20:30], payload)
	return out
}

func testWebPVP8(width int, height int) []byte {
	payload := make([]byte, 10)
	payload[3] = 0x9d
	payload[4] = 0x01
	payload[5] = 0x2a
	payload[6] = byte(width)
	payload[7] = byte(width >> 8)
	payload[8] = byte(height)
	payload[9] = byte(height >> 8)
	return testWebPChunk("VP8 ", payload)
}

func testWebPVP8L(width int, height int) []byte {
	payload := make([]byte, 5)
	payload[0] = 0x2f
	bits := uint32(width-1) | uint32(height-1)<<14
	payload[1] = byte(bits)
	payload[2] = byte(bits >> 8)
	payload[3] = byte(bits >> 16)
	payload[4] = byte(bits >> 24)
	return testWebPChunk("VP8L", payload)
}

func testWebPChunk(kind string, payload []byte) []byte {
	size := len(payload)
	out := make([]byte, 20+size+(size&1))
	copy(out[0:4], "RIFF")
	riffSize := len(out) - 8
	out[4] = byte(riffSize)
	out[5] = byte(riffSize >> 8)
	out[6] = byte(riffSize >> 16)
	out[7] = byte(riffSize >> 24)
	copy(out[8:12], "WEBP")
	copy(out[12:16], kind)
	out[16] = byte(size)
	out[17] = byte(size >> 8)
	out[18] = byte(size >> 16)
	out[19] = byte(size >> 24)
	copy(out[20:], payload)
	return out
}
