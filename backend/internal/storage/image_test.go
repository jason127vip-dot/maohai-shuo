package storage

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestNormalizeAndSquareOutput(t *testing.T) {
	var source bytes.Buffer
	_ = png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 2560, 1440)))
	data, err := Normalize(source.Bytes(), 20000000, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != 2048 || cfg.Height != 1152 {
		t.Fatal("input resize mismatch")
	}
	renderer, err := NewRenderer("", 512, "毛孩儿说", true)
	if err != nil {
		t.Fatal(err)
	}
	result, err := renderer.Render(data)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err = jpeg.DecodeConfig(bytes.NewReader(result))
	if err != nil || cfg.Width != 512 || cfg.Height != 512 {
		t.Fatal("output is not a square JPEG")
	}
}
func TestRejectInvalidImages(t *testing.T) {
	if _, err := Normalize([]byte("not an image"), 20000000, 2048); err == nil {
		t.Fatal("invalid image accepted")
	}
	var tiny bytes.Buffer
	_ = png.Encode(&tiny, image.NewRGBA(image.Rect(0, 0, 32, 32)))
	if _, err := Normalize(tiny.Bytes(), 20000000, 2048); err == nil {
		t.Fatal("tiny image accepted")
	}
	if _, err := NewRenderer("", 512, "毛孩儿说", false); err == nil {
		t.Fatal("real generation allowed a font with no Chinese glyphs")
	}
}

func TestPetCaptionIsDrawnInsideSquareResult(t *testing.T) {
	var source bytes.Buffer
	_ = png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 256, 256)))
	renderer, err := NewRenderer("", 512, "demo", true)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := renderer.Render(source.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	captioned, err := renderer.RenderWithCaption(source.Bytes(), "Milo - 2026.11.19")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(plain, captioned) {
		t.Fatal("caption did not affect output image")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(captioned))
	if err != nil || cfg.Width != 512 || cfg.Height != 512 {
		t.Fatal("caption changed output dimensions")
	}
	if _, err = renderer.RenderWithCaption(source.Bytes(), "奶茶"); err == nil {
		t.Fatal("missing glyphs were silently rendered")
	}
}
