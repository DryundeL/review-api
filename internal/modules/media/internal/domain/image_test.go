package domain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"review-api/internal/modules/media/contract"
)

const gpsMarker = "GPSLatitude=55.7558"

func sample(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.RGBA{uint8(x * 10), uint8(y * 10), 128, 255})
		}
	}
	return img
}

func jpegSegment(marker byte, payload string) []byte {
	seg := []byte{0xFF, marker, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	return append(seg, payload...)
}

// jpegWithMetadata inserts EXIF, XMP and COM right after SOI and garbage after EOI.
func jpegWithMetadata(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, sample(w, h), nil); err != nil {
		t.Fatal(err)
	}
	src := buf.Bytes()
	out := append([]byte{}, src[:2]...)
	out = append(out, jpegSegment(0xE1, "Exif\x00\x00"+gpsMarker)...)
	out = append(out, jpegSegment(0xE1, "http://ns.adobe.com/xap/1.0/\x00"+gpsMarker)...)
	out = append(out, jpegSegment(0xE2, "ICC_PROFILE\x00\x01\x01profile")...)
	out = append(out, jpegSegment(0xFE, gpsMarker)...)
	out = append(out, src[2:]...)
	return append(out, []byte("trailing "+gpsMarker)...)
}

func pngChunk(typ, payload string) []byte {
	out := make([]byte, 4, 12+len(payload))
	binary.BigEndian.PutUint32(out, uint32(len(payload)))
	out = append(out, typ...)
	out = append(out, payload...)
	crc := crc32.ChecksumIEEE([]byte(typ + payload))
	return binary.BigEndian.AppendUint32(out, crc)
}

// pngWithMetadata inserts text/EXIF/private chunks before IEND and garbage after it.
func pngWithMetadata(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, sample(w, h)); err != nil {
		t.Fatal(err)
	}
	src := buf.Bytes()
	iend := len(src) - 12
	out := append([]byte{}, src[:iend]...)
	out = append(out, pngChunk("tEXt", "Comment\x00"+gpsMarker)...)
	out = append(out, pngChunk("eXIf", gpsMarker)...)
	out = append(out, pngChunk("prVt", gpsMarker)...)
	out = append(out, src[iend:]...)
	return append(out, gpsMarker...)
}

func riffChunk(fourcc string, payload []byte) []byte {
	out := append([]byte(fourcc), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(payload)))
	out = append(out, payload...)
	if len(payload)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

func vp8xPayload(w, h int, flags byte) []byte {
	p := make([]byte, 10)
	p[0] = flags
	p[4], p[5], p[6] = byte(w-1), byte((w-1)>>8), byte((w-1)>>16)
	p[7], p[8], p[9] = byte(h-1), byte((h-1)>>8), byte((h-1)>>16)
	return p
}

func vp8lPayload(w, h int) []byte {
	bits := uint32(w-1) | uint32(h-1)<<14
	p := []byte{0x2F, 0, 0, 0, 0, 0xAA, 0xBB}
	binary.LittleEndian.PutUint32(p[1:5], bits)
	return p
}

func webp(chunks ...[]byte) []byte {
	body := []byte("WEBP")
	for _, c := range chunks {
		body = append(body, c...)
	}
	out := append([]byte("RIFF"), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(body)))
	return append(out, body...)
}

func TestProcessJPEGStripsMetadata(t *testing.T) {
	t.Parallel()
	img, err := Process(jpegWithMetadata(t, 12, 7))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if img.Format != FormatJPEG || img.Width != 12 || img.Height != 7 {
		t.Fatalf("got %s %dx%d", img.Format, img.Width, img.Height)
	}
	if bytes.Contains(img.Data, []byte(gpsMarker)) {
		t.Fatal("metadata survived")
	}
	if !bytes.Contains(img.Data, []byte("ICC_PROFILE")) {
		t.Fatal("ICC profile must be kept")
	}
	if !bytes.HasSuffix(img.Data, []byte{0xFF, 0xD9}) {
		t.Fatal("output must end at EOI")
	}
	if _, err := jpeg.Decode(bytes.NewReader(img.Data)); err != nil {
		t.Fatalf("stripped JPEG does not decode: %v", err)
	}
}

func TestProcessJPEGStripsMetadataBetweenProgressiveScans(t *testing.T) {
	t.Parallel()
	src := jpegWithMetadata(t, 4, 4)
	eoi := bytes.LastIndex(src, []byte{0xFF, 0xD9})
	// An APP1 segment after the scan data, before EOI, must also be dropped.
	data := append(append(append([]byte{}, src[:eoi]...), jpegSegment(0xE1, "Exif\x00\x00"+gpsMarker)...), 0xFF, 0xD9)
	img, err := Process(data)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if bytes.Contains(img.Data, []byte(gpsMarker)) {
		t.Fatal("metadata after scan survived")
	}
}

func TestProcessPNGStripsMetadata(t *testing.T) {
	t.Parallel()
	img, err := Process(pngWithMetadata(t, 9, 5))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if img.Format != FormatPNG || img.Width != 9 || img.Height != 5 {
		t.Fatalf("got %s %dx%d", img.Format, img.Width, img.Height)
	}
	if bytes.Contains(img.Data, []byte(gpsMarker)) {
		t.Fatal("metadata survived")
	}
	if _, err := png.Decode(bytes.NewReader(img.Data)); err != nil {
		t.Fatalf("stripped PNG does not decode: %v", err)
	}
}

func TestProcessWebPStripsMetadata(t *testing.T) {
	t.Parallel()
	data := webp(
		riffChunk("VP8X", vp8xPayload(300, 200, 0x10|vp8xFlagEXIF|vp8xFlagXMP)),
		riffChunk("ICCP", []byte("icc")),
		riffChunk("VP8L", vp8lPayload(300, 200)),
		riffChunk("EXIF", []byte(gpsMarker)),
		riffChunk("XMP ", []byte(gpsMarker)),
		riffChunk("ZZZZ", []byte(gpsMarker)),
	)
	data = append(data, gpsMarker...) // outside the RIFF container
	img, err := Process(data)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if img.Format != FormatWebP || img.Width != 300 || img.Height != 200 {
		t.Fatalf("got %s %dx%d", img.Format, img.Width, img.Height)
	}
	if bytes.Contains(img.Data, []byte(gpsMarker)) {
		t.Fatal("metadata survived")
	}
	if got := binary.LittleEndian.Uint32(img.Data[4:8]); int(got) != len(img.Data)-8 {
		t.Fatalf("RIFF size %d, want %d", got, len(img.Data)-8)
	}
	if flags := img.Data[20]; flags != 0x10 {
		t.Fatalf("VP8X flags = %#x, want only alpha", flags)
	}
	if !bytes.Contains(img.Data, []byte("ICCP")) {
		t.Fatal("ICC profile must be kept")
	}
}

func TestProcessWebPLossySize(t *testing.T) {
	t.Parallel()
	payload := []byte{0x10, 0x02, 0x00, 0x9D, 0x01, 0x2A, 0x40, 0x01, 0xF0, 0x00, 0x01, 0x02}
	img, err := Process(webp(riffChunk("VP8 ", payload)))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if img.Width != 320 || img.Height != 240 {
		t.Fatalf("got %dx%d", img.Width, img.Height)
	}
}

func TestProcessRejects(t *testing.T) {
	t.Parallel()
	validJPEG := jpegWithMetadata(t, 4, 4)
	validPNG := pngWithMetadata(t, 4, 4)
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"empty", nil, contract.ErrInvalid},
		{"gif", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), contract.ErrUnsupportedFormat},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), contract.ErrUnsupportedFormat},
		{"html", []byte("<!DOCTYPE html><html>"), contract.ErrUnsupportedFormat},
		{"too large", append(append([]byte{}, validJPEG...), make([]byte, contract.MaxImageBytes)...), contract.ErrTooLarge},
		{"truncated jpeg", validJPEG[:len(validJPEG)/2], contract.ErrInvalid},
		{"jpeg without scan", []byte{0xFF, 0xD8, 0xFF, 0xD9}, contract.ErrInvalid},
		{"jpeg bad segment length", []byte{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF, 0x00}, contract.ErrInvalid},
		{"truncated png", validPNG[:40], contract.ErrInvalid},
		{"png chunk length overflow", append(append([]byte{}, pngSignature...), 0xFF, 0xFF, 0xFF, 0xFF, 'I', 'H', 'D', 'R', 0, 0, 0, 0), contract.ErrInvalid},
		{"png unknown critical chunk", append(append([]byte{}, validPNG[:33]...), pngChunk("ABCD", "")...), contract.ErrInvalid},
		{"webp riff size overflow", []byte("RIFF\xff\xff\xff\x7fWEBPVP8L"), contract.ErrInvalid},
		{"webp no image chunk", webp(riffChunk("VP8X", vp8xPayload(10, 10, 0))), contract.ErrInvalid},
		{"webp unknown first chunk", webp(riffChunk("EXIF", []byte(gpsMarker)), riffChunk("VP8L", vp8lPayload(1, 1))), contract.ErrInvalid},
		{"webp chunk overflow", webp([]byte("VP8L\xff\xff\x00\x00\x2f")), contract.ErrInvalid},
		{"webp too wide", webp(riffChunk("VP8X", vp8xPayload(contract.MaxImageSide+1, 10, 0)), riffChunk("VP8L", vp8lPayload(1, 1))), contract.ErrDimensions},
		{"png too tall", mustPNG(t, 1, contract.MaxImageSide+1), contract.ErrDimensions},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Process(tc.data); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func mustPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestProcessSizeBoundary(t *testing.T) {
	t.Parallel()
	base := mustPNG(t, 2, 2)
	pad := func(total int) []byte {
		// Pad with a private ancillary chunk placed before IEND; it is dropped on output.
		iend := len(base) - 12
		filler := total - len(base) - 12
		out := append([]byte{}, base[:iend]...)
		out = append(out, pngChunk("prVt", string(make([]byte, filler)))...)
		return append(out, base[iend:]...)
	}
	if _, err := Process(pad(contract.MaxImageBytes)); err != nil {
		t.Fatalf("exactly at limit: %v", err)
	}
	if _, err := Process(pad(contract.MaxImageBytes + 1)); !errors.Is(err, contract.ErrTooLarge) {
		t.Fatalf("over limit: %v", err)
	}
}

func FuzzProcess(f *testing.F) {
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xDA, 0x00, 0x02, 0xFF, 0xD9})
	f.Add([]byte("RIFF\x10\x00\x00\x00WEBPVP8L\x05\x00\x00\x00\x2f\x00\x00\x00\x00\x00"))
	f.Add(append(append([]byte{}, pngSignature...), pngChunk("IHDR", "\x00\x00\x00\x01\x00\x00\x00\x01\x08\x00\x00\x00\x00")...))
	f.Fuzz(func(t *testing.T, data []byte) {
		img, err := Process(data)
		if err != nil {
			return
		}
		if len(img.Data) > len(data) {
			t.Fatalf("output grew: %d > %d", len(img.Data), len(data))
		}
	})
}
