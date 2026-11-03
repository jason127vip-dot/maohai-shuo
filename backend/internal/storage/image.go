package storage

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"os"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
)

func Normalize(data []byte, maxPixels int64, maxEdge int) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") {
		return nil, errors.New("请选择有效的 JPEG、PNG 或 WebP 图片")
	}
	if cfg.Width < 128 || cfg.Height < 128 || int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return nil, errors.New("照片太小或分辨率过大，请更换照片")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("图片内容损坏，请更换照片")
	}
	width, height := cfg.Width, cfg.Height
	if width > maxEdge || height > maxEdge {
		if width >= height {
			height = max(1, height*maxEdge/width)
			width = maxEdge
		} else {
			width = max(1, width*maxEdge/height)
			height = maxEdge
		}
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	xdraw.ApproxBiLinear.Scale(canvas, canvas.Bounds(), img, img.Bounds(), draw.Over, nil)
	var out bytes.Buffer
	if err = jpeg.Encode(&out, canvas, &jpeg.Options{Quality: 90}); err != nil {
		return nil, errors.New("图片处理失败")
	}
	return out.Bytes(), nil
}

type Renderer struct {
	font  *opentype.Font
	size  int
	brand string
	demo  bool
}

func NewRenderer(fontPath string, size int, brand string, demo bool) (*Renderer, error) {
	data := goregular.TTF
	if fontPath != "" {
		var err error
		data, err = os.ReadFile(fontPath)
		if err != nil {
			return nil, errors.New("角标字体无法读取")
		}
	}
	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil, errors.New("角标字体格式无效，请使用 TTF 或 OTF")
	}
	if !demo {
		for _, r := range brand + "AI 生成" {
			glyph, err := parsed.GlyphIndex(nil, r)
			if err != nil || glyph == 0 {
				return nil, errors.New("角标字体缺少所需中文字形")
			}
		}
	}
	return &Renderer{parsed, size, brand, demo}, nil
}
func (r *Renderer) Render(data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 40000000 {
		return nil, errors.New("模型返回图片无效")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("模型返回图片无法解码")
	}
	canvas := image.NewRGBA(image.Rect(0, 0, r.size, r.size))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{255, 251, 244, 255}), image.Point{}, draw.Src)
	margin := r.size / 40
	band := r.size / 14
	available := r.size - margin*2 - band
	width, height := cfg.Width, cfg.Height
	if width >= height {
		height = max(1, height*(r.size-margin*2)/width)
		width = r.size - margin*2
	} else {
		width = max(1, width*available/height)
		height = available
	}
	if height > available {
		width = max(1, width*available/height)
		height = available
	}
	target := image.Rect((r.size-width)/2, margin+(available-height)/2, (r.size+width)/2, margin+(available+height)/2)
	xdraw.CatmullRom.Scale(canvas, target, img, img.Bounds(), draw.Over, nil)
	face, err := opentype.NewFace(r.font, &opentype.FaceOptions{Size: float64(r.size) / 42, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, errors.New("角标字体初始化失败")
	}
	defer face.Close()
	label := r.brand + " · AI 生成"
	if r.demo {
		label = "MAOHAI SHUO - DEMO / ORIGINAL PHOTO"
	}
	drawer := font.Drawer{Dst: canvas, Src: image.NewUniform(color.RGBA{135, 78, 52, 255}), Face: face}
	widthText := drawer.MeasureString(label).Ceil()
	x := max(margin, r.size-margin-widthText)
	drawer.Dot = fixed.P(x, r.size-margin)
	drawer.DrawString(label)
	var out bytes.Buffer
	if jpeg.Encode(&out, canvas, &jpeg.Options{Quality: 92}) != nil {
		return nil, errors.New("成品图片编码失败")
	}
	return out.Bytes(), nil
}
