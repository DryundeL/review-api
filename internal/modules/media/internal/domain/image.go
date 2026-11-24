// Package domain validates uploaded images by content and removes metadata
// (EXIF, XMP, text chunks) without re-encoding pixels.
//
// Every format is filtered by an allowlist of structural pieces: anything
// unknown is dropped, so new metadata containers cannot slip through.
package domain

import (
	"bytes"
	"encoding/binary"
	"image"
	_ "image/jpeg" // registers DecodeConfig for JPEG
	_ "image/png"  // registers DecodeConfig for PNG

	"review-api/internal/modules/media/contract"
)

type Format string

const (
	FormatJPEG Format = "image/jpeg"
	FormatPNG  Format = "image/png"
	FormatWebP Format = "image/webp"
)

// Ext is the file extension used in storage keys.
func (f Format) Ext() string {
	switch f {
	case FormatJPEG:
		return "jpg"
	case FormatPNG:
		return "png"
	case FormatWebP:
		return "webp"
	default:
		return ""
	}
}

// Image is a validated upload with metadata removed.
type Image struct {
	Format Format
	Data   []byte
	Width  int
	Height int
}

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// Sniff detects the format by magic bytes. File name and declared MIME type are never used.
func Sniff(data []byte) (Format, error) {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return FormatJPEG, nil
	case bytes.HasPrefix(data, pngSignature):
		return FormatPNG, nil
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return FormatWebP, nil
	default:
		return "", contract.ErrUnsupportedFormat
	}
}

// Process checks size, format and dimensions and returns a copy without metadata.
func Process(data []byte) (Image, error) {
	if len(data) == 0 {
		return Image{}, contract.ErrInvalid
	}
	if len(data) > contract.MaxImageBytes {
		return Image{}, contract.ErrTooLarge
	}
	format, err := Sniff(data)
	if err != nil {
		return Image{}, err
	}

	var (
		clean []byte
		w, h  int
	)
	switch format {
	case FormatJPEG:
		clean, err = stripJPEG(data)
		if err == nil {
			w, h, err = decodeConfig(clean)
		}
	case FormatPNG:
		clean, err = stripPNG(data)
		if err == nil {
			w, h, err = decodeConfig(clean)
		}
	case FormatWebP:
		clean, w, h, err = stripWebP(data)
	}
	if err != nil {
		return Image{}, err
	}
	if w < 1 || h < 1 || w > contract.MaxImageSide || h > contract.MaxImageSide {
		return Image{}, contract.ErrDimensions
	}
	return Image{Format: format, Data: clean, Width: w, Height: h}, nil
}

func decodeConfig(data []byte) (int, int, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, contract.ErrInvalid
	}
	return cfg.Width, cfg.Height, nil
}

// stripJPEG keeps APP0 (JFIF), APP2 ICC_PROFILE and APP14 Adobe (needed for CMYK colours)
// and drops every other APPn and COM segment, including those between progressive scans.
// Bytes after EOI are discarded.
func stripJPEG(data []byte) ([]byte, error) {
	out := make([]byte, 0, len(data))
	out = append(out, 0xFF, 0xD8)
	i := 2
	sawScan := false
	for {
		if i >= len(data) || data[i] != 0xFF {
			return nil, contract.ErrInvalid
		}
		for i < len(data) && data[i] == 0xFF { // fill bytes
			i++
		}
		if i >= len(data) {
			return nil, contract.ErrInvalid
		}
		marker := data[i]
		i++
		switch {
		case marker == 0xD9:
			if !sawScan {
				return nil, contract.ErrInvalid
			}
			return append(out, 0xFF, 0xD9), nil
		case marker == 0x00 || marker == 0xD8:
			return nil, contract.ErrInvalid
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7):
			out = append(out, 0xFF, marker)
			continue
		}
		if i+2 > len(data) {
			return nil, contract.ErrInvalid
		}
		n := int(binary.BigEndian.Uint16(data[i : i+2]))
		if n < 2 || i+n > len(data) {
			return nil, contract.ErrInvalid
		}
		segment := data[i : i+n]
		i += n
		if keepJPEGSegment(marker, segment[2:]) {
			out = append(out, 0xFF, marker)
			out = append(out, segment...)
		}
		if marker != 0xDA {
			continue
		}
		// Entropy-coded data: FF is followed by 00 (stuffing) or RSTn; any other byte starts a marker.
		sawScan = true
		start := i
		for i < len(data) {
			if data[i] == 0xFF && i+1 < len(data) {
				next := data[i+1]
				if next == 0x00 || (next >= 0xD0 && next <= 0xD7) {
					i += 2
					continue
				}
				break
			}
			i++
		}
		out = append(out, data[start:i]...)
	}
}

func keepJPEGSegment(marker byte, payload []byte) bool {
	switch {
	case marker == 0xFE: // COM
		return false
	case marker == 0xE0:
		return true
	case marker == 0xE2:
		return bytes.HasPrefix(payload, []byte("ICC_PROFILE\x00"))
	case marker == 0xEE:
		return bytes.HasPrefix(payload, []byte("Adobe"))
	case marker >= 0xE0 && marker <= 0xEF:
		return false
	default:
		return true
	}
}

// PNG chunks that affect rendering. Everything else ancillary (tEXt, zTXt, iTXt, eXIf, tIME,
// private chunks) is dropped. Unknown critical chunks make the file invalid.
var pngKeep = map[string]bool{
	"IHDR": true, "PLTE": true, "IDAT": true, "IEND": true,
	"tRNS": true, "cHRM": true, "gAMA": true, "iCCP": true, "sBIT": true,
	"sRGB": true, "cICP": true, "bKGD": true, "pHYs": true,
	"acTL": true, "fcTL": true, "fdAT": true,
}

func stripPNG(data []byte) ([]byte, error) {
	out := make([]byte, 0, len(data))
	out = append(out, pngSignature...)
	i := len(pngSignature)
	first := true
	for i+12 <= len(data) {
		length := binary.BigEndian.Uint32(data[i : i+4])
		if uint64(length) > uint64(len(data)-i-12) {
			return nil, contract.ErrInvalid
		}
		end := i + 12 + int(length)
		typ := string(data[i+4 : i+8])
		if first && typ != "IHDR" {
			return nil, contract.ErrInvalid
		}
		first = false
		critical := typ[0] >= 'A' && typ[0] <= 'Z'
		switch {
		case pngKeep[typ]:
			out = append(out, data[i:end]...)
		case critical:
			return nil, contract.ErrInvalid
		}
		if typ == "IEND" {
			return out, nil
		}
		i = end
	}
	return nil, contract.ErrInvalid
}

// WebP chunks that affect rendering; EXIF, XMP and unknown chunks are dropped.
var webpKeep = map[string]bool{
	"VP8 ": true, "VP8L": true, "VP8X": true, "ALPH": true,
	"ANIM": true, "ANMF": true, "ICCP": true,
}

const (
	vp8xFlagEXIF = 0x08
	vp8xFlagXMP  = 0x04
)

func stripWebP(data []byte) ([]byte, int, int, error) {
	riffSize := binary.LittleEndian.Uint32(data[4:8])
	if riffSize < 4 || uint64(riffSize) > uint64(len(data)-8) {
		return nil, 0, 0, contract.ErrInvalid
	}
	end := 8 + int(riffSize) // bytes after the RIFF container are discarded

	out := make([]byte, 12, len(data))
	copy(out, data[:12])
	i := 12
	flagsAt := -1
	var w, h int
	hasImage := false
	for i < end {
		if i+8 > end {
			return nil, 0, 0, contract.ErrInvalid
		}
		fourcc := string(data[i : i+4])
		size := binary.LittleEndian.Uint32(data[i+4 : i+8])
		if uint64(size) > uint64(end-i-8) {
			return nil, 0, 0, contract.ErrInvalid
		}
		payload := data[i+8 : i+8+int(size)]
		next := i + 8 + int(size) + int(size&1)
		if next > end {
			return nil, 0, 0, contract.ErrInvalid
		}
		if i == 12 {
			var err error
			if w, h, err = webpSize(fourcc, payload); err != nil {
				return nil, 0, 0, err
			}
		}
		switch fourcc {
		case "VP8 ", "VP8L", "ANMF":
			hasImage = true
		}
		if webpKeep[fourcc] {
			if fourcc == "VP8X" && flagsAt < 0 {
				flagsAt = len(out) + 8
			}
			out = append(out, data[i:next]...)
		}
		i = next
	}
	if !hasImage {
		return nil, 0, 0, contract.ErrInvalid
	}
	if flagsAt >= 0 {
		out[flagsAt] &^= vp8xFlagEXIF | vp8xFlagXMP
	}
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out, w, h, nil
}

// webpSize reads the canvas size from the first chunk, which must describe the image.
func webpSize(fourcc string, p []byte) (int, int, error) {
	switch fourcc {
	case "VP8X":
		if len(p) < 10 {
			return 0, 0, contract.ErrInvalid
		}
		w := 1 + int(uint32(p[4])|uint32(p[5])<<8|uint32(p[6])<<16)
		h := 1 + int(uint32(p[7])|uint32(p[8])<<8|uint32(p[9])<<16)
		return w, h, nil
	case "VP8L":
		if len(p) < 5 || p[0] != 0x2F {
			return 0, 0, contract.ErrInvalid
		}
		bits := binary.LittleEndian.Uint32(p[1:5])
		return 1 + int(bits&0x3FFF), 1 + int((bits>>14)&0x3FFF), nil
	case "VP8 ":
		if len(p) < 10 || p[3] != 0x9D || p[4] != 0x01 || p[5] != 0x2A {
			return 0, 0, contract.ErrInvalid
		}
		w := int(binary.LittleEndian.Uint16(p[6:8]) & 0x3FFF)
		h := int(binary.LittleEndian.Uint16(p[8:10]) & 0x3FFF)
		return w, h, nil
	default:
		return 0, 0, contract.ErrInvalid
	}
}
