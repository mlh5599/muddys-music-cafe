// Package uploads validates, normalises and stores uploaded pictures.
//
// Uploads are untrusted: the type is decided by the file's content (never its
// name), size and pixel counts are capped, the picture is decoded and
// re-encoded (which drops any metadata and anything hidden in the file), large
// pictures are scaled down, phone rotation is applied, and the stored filename
// is random.
package uploads

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"

	_ "image/gif" // register GIF decoding
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register WebP decoding
)

const (
	// MaxBytes is the largest upload accepted.
	MaxBytes = 10 << 20
	// MaxPixels caps width*height so a tiny file can't expand to gigabytes.
	MaxPixels = 40_000_000
	// MaxSide is the longest edge stored; larger pictures are scaled down.
	MaxSide = 1600
)

// Errors carry plain-language text that is safe to show to the user.
var (
	ErrTooLarge = errors.New("That picture is too big. Please choose one under 10 MB.")
	ErrNotImage = errors.New("That file isn't a picture we can use. Please choose a JPG or PNG photo.")
	ErrTooMany  = errors.New("That picture has too many pixels. Please choose a smaller one.")
)

// Result is a processed picture ready to store.
type Result struct {
	Data          []byte
	ContentType   string
	Ext           string
	Width, Height int
}

// Process validates and normalises an uploaded picture.
func Process(r io.Reader) (Result, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return Result{}, fmt.Errorf("uploads: read: %w", err)
	}
	if len(raw) > MaxBytes {
		return Result{}, ErrTooLarge
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return Result{}, ErrNotImage
	}
	if cfg.Width*cfg.Height > MaxPixels {
		return Result{}, ErrTooMany
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return Result{}, ErrNotImage
	}

	img = scaleDown(img, MaxSide)
	if format == "jpeg" {
		img = orient(img, exifOrientation(raw))
	}

	var buf bytes.Buffer
	res := Result{Width: img.Bounds().Dx(), Height: img.Bounds().Dy()}
	if isOpaque(img) {
		res.ContentType, res.Ext = "image/jpeg", ".jpg"
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	} else {
		res.ContentType, res.Ext = "image/png", ".png"
		err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, img)
	}
	if err != nil {
		return Result{}, fmt.Errorf("uploads: encode: %w", err)
	}
	res.Data = buf.Bytes()
	return res, nil
}

// Store writes the processed picture into dir under a random filename and
// returns that name. The write is atomic.
func Store(dir string, res Result) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("uploads: create dir: %w", err)
	}
	var rnd [16]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", fmt.Errorf("uploads: random name: %w", err)
	}
	name := hex.EncodeToString(rnd[:]) + res.Ext

	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return "", fmt.Errorf("uploads: temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(res.Data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("uploads: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("uploads: close: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o640); err != nil {
		return "", fmt.Errorf("uploads: chmod: %w", err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, name)); err != nil {
		return "", fmt.Errorf("uploads: rename: %w", err)
	}
	return name, nil
}

// Remove deletes a stored file; a missing file is not an error.
func Remove(dir, name string) error {
	if name == "" || name != filepath.Base(name) {
		return nil
	}
	if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("uploads: remove: %w", err)
	}
	return nil
}

func scaleDown(img image.Image, maxSide int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxSide && h <= maxSide {
		return img
	}
	if w >= h {
		h, w = h*maxSide/w, maxSide
	} else {
		w, h = w*maxSide/h, maxSide
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

// orient applies an EXIF orientation (1-8) so the picture is upright.
func orient(img image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			case 8:
				nx, ny = y, w-1-x
			}
			dst.Set(nx, ny, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func isOpaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return false
			}
		}
	}
	return true
}
