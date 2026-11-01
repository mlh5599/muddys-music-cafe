package uploads

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func solid(w, h int, c color.Color) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestProcessJPEG(t *testing.T) {
	res, err := Process(bytes.NewReader(encodeJPEG(t, solid(300, 200, color.White))))
	if err != nil {
		t.Fatal(err)
	}
	if res.ContentType != "image/jpeg" || res.Ext != ".jpg" || res.Width != 300 || res.Height != 200 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestProcessKeepsTransparencyAsPNG(t *testing.T) {
	res, err := Process(bytes.NewReader(encodePNG(t, solid(10, 10, color.NRGBA{255, 0, 0, 128}))))
	if err != nil {
		t.Fatal(err)
	}
	if res.ContentType != "image/png" || res.Ext != ".png" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestProcessScalesDown(t *testing.T) {
	res, err := Process(bytes.NewReader(encodeJPEG(t, solid(3200, 1600, color.White))))
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != MaxSide || res.Height != MaxSide/2 {
		t.Fatalf("got %dx%d", res.Width, res.Height)
	}
}

func TestProcessRejectsNonImages(t *testing.T) {
	for name, data := range map[string][]byte{
		"text":   []byte("hello"),
		"script": []byte("<script>alert(1)</script>"),
		"empty":  nil,
		"svg":    []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"trunc":  encodeJPEG(t, solid(50, 50, color.White))[:100],
	} {
		if _, err := Process(bytes.NewReader(data)); err != ErrNotImage {
			t.Errorf("%s: err = %v, want ErrNotImage", name, err)
		}
	}
}

func TestProcessRejectsTooLarge(t *testing.T) {
	if _, err := Process(bytes.NewReader(make([]byte, MaxBytes+1))); err != ErrTooLarge {
		t.Fatalf("err = %v", err)
	}
}

func TestProcessStripsTrailingData(t *testing.T) {
	data := append(encodeJPEG(t, solid(20, 20, color.White)), []byte("<?php evil ?>")...)
	res, err := Process(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(res.Data, []byte("evil")) {
		t.Fatal("trailing data survived re-encoding")
	}
}

func TestOrientRotates(t *testing.T) {
	// 2 wide x 1 tall; orientation 6 (rotate 90° CW) must become 1 wide x 2 tall.
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	src.Set(1, 0, color.NRGBA{0, 0, 255, 255})
	out := orient(src, 6)
	if out.Bounds().Dx() != 1 || out.Bounds().Dy() != 2 {
		t.Fatalf("bounds = %v", out.Bounds())
	}
	if r, _, _, _ := out.At(0, 0).RGBA(); r == 0 {
		t.Fatal("red pixel should be at the top after a 90° clockwise turn")
	}
}

func TestExifOrientation(t *testing.T) {
	// SOI, APP1 "Exif" with a little-endian TIFF holding orientation=6.
	tiff := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 0x01, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	seg := append([]byte("Exif\x00\x00"), tiff...)
	b := []byte{0xFF, 0xD8, 0xFF, 0xE1, byte((len(seg) + 2) >> 8), byte(len(seg) + 2)}
	b = append(b, seg...)
	b = append(b, 0xFF, 0xDA)
	if got := exifOrientation(b); got != 6 {
		t.Fatalf("orientation = %d", got)
	}
	if got := exifOrientation([]byte("junk")); got != 1 {
		t.Fatalf("junk orientation = %d", got)
	}
}

func TestStoreAndRemove(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "uploads")
	res := Result{Data: []byte("x"), Ext: ".jpg"}
	a, err := Store(dir, res)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Store(dir, res)
	if a == b || !strings.HasSuffix(a, ".jpg") || len(a) != 32+4 {
		t.Fatalf("names %q %q", a, b)
	}
	if _, err := os.Stat(filepath.Join(dir, a)); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Fatalf("leftover temp files: %d entries", len(entries))
	}
	if err := Remove(dir, a); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir, a); err != nil {
		t.Fatal("removing a missing file should not fail")
	}
	if err := Remove(dir, "../escape"); err != nil {
		t.Fatal(err)
	}
}
