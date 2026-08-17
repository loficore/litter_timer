// Package handlers — wallpaper image processing (UUID naming + compression).
//
// This file provides the UUID filename generator and the image decode/scale/encode
// pipeline used by both WallpaperUpload and WallpaperFromURL.
package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

var (
	// ErrWallpaperDimensionsTooLarge is returned when either dimension exceeds 12000px.
	ErrWallpaperDimensionsTooLarge = errors.New("wallpaper dimensions exceed 12000px")
	// ErrWallpaperDecodeFailed is returned when the image cannot be decoded.
	ErrWallpaperDecodeFailed = errors.New("failed to decode wallpaper image")
	// ErrWallpaperUnsupportedFormat is returned for unrecognised image formats.
	ErrWallpaperUnsupportedFormat = errors.New("unsupported wallpaper format")
)

// newUUIDHex returns a 32-character lowercase hex string from crypto/rand.
// No third-party UUID library is used.
func newUUIDHex() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// processWallpaperImage reads raw image bytes, optionally decodes and scales
// them, then re-encodes to the output format.  Returns the encoded bytes, the
// output file extension, and any error.
//
// Behaviour by input extension:
//
//	.jpg / .jpeg / .png / .webp → decode, scale if long edge >2560, re-encode
//	.gif / .svg / .bmp            → passthrough (return original bytes unchanged)
func processWallpaperImage(src io.Reader, ext string) ([]byte, string, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, "", fmt.Errorf("reading image: %w", err)
	}

	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
		return processDecodeEncode(data, ext)
	case ".gif", ".svg", ".bmp":
		return data, ext, nil
	default:
		return nil, "", ErrWallpaperUnsupportedFormat
	}
}

// processDecodeEncode handles the decode → scale → encode pipeline for
// formats that need re-compression.
func processDecodeEncode(data []byte, ext string) ([]byte, string, error) {
	// 1. DecodeConfig — check dimensions before decoding the full image.
	cfg, err := decodeConfig(bytes.NewReader(data), ext)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrWallpaperDecodeFailed, err)
	}
	if cfg.Width > 12000 || cfg.Height > 12000 {
		return nil, "", ErrWallpaperDimensionsTooLarge
	}

	// 2. Decode the full image.
	img, err := decodeFull(bytes.NewReader(data), ext)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrWallpaperDecodeFailed, err)
	}

	// 3. Scale down if the long edge exceeds 2560px (preserving aspect ratio).
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	longEdge := w
	if h > w {
		longEdge = h
	}
	if longEdge > 2560 {
		scale := 2560.0 / float64(longEdge)
		newW := int(float64(w) * scale)
		newH := int(float64(h) * scale)
		if newW < 1 {
			newW = 1
		}
		if newH < 1 {
			newH = 1
		}
		scaled := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), img, bounds, draw.Over, nil)
		img = scaled
	}

	// 4. Encode to the appropriate output format.
	var buf bytes.Buffer
	var outExt string
	switch ext {
	case ".jpg", ".jpeg", ".webp":
		// JPEG output for all three — webp source is transcoded.
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			return nil, "", fmt.Errorf("encoding jpeg: %w", err)
		}
		outExt = ".jpg"
	case ".png":
		// PNG output preserves alpha.
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", fmt.Errorf("encoding png: %w", err)
		}
		outExt = ".png"
	default:
		return nil, "", ErrWallpaperUnsupportedFormat
	}

	return buf.Bytes(), outExt, nil
}

// decodeConfig reads image dimensions.  Uses the webp package directly for
// .webp since it is not registered with the standard image package.
func decodeConfig(r io.Reader, ext string) (image.Config, error) {
	switch ext {
	case ".webp":
		return webp.DecodeConfig(r)
	default:
		cfg, _, err := image.DecodeConfig(r)
		return cfg, err
	}
}

// decodeFull decodes the full image.  Uses the webp package directly for .webp.
func decodeFull(r io.Reader, ext string) (image.Image, error) {
	switch ext {
	case ".webp":
		return webp.Decode(r)
	default:
		img, _, err := image.Decode(r)
		return img, err
	}
}