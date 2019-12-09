package render

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/unidoc/unipdf/v3/model"
	imagectx "github.com/unidoc/unipdf/v3/render/context/image"
)

// ImageDevice is used to render PDF pages to image targets.
type ImageDevice struct {
	renderer
}

// NewImageDevice returns a new image device.
func NewImageDevice() *ImageDevice {
	return &ImageDevice{}
}

func (d *ImageDevice) render(page *model.PdfPage) (*imagectx.Context, error) {
	mbox, err := page.GetMediaBox()
	if err != nil {
		return nil, err
	}

	ctx := imagectx.NewContext(int(mbox.Width()), int(mbox.Height()))
	if err := d.renderPage(ctx, page); err != nil {
		return nil, err
	}

	return ctx, nil
}

// Render converts the specified PDF page into an image and returns the result.
func (d *ImageDevice) Render(page *model.PdfPage) (image.Image, error) {
	ctx, err := d.render(page)
	if err != nil {
		return nil, err
	}

	return ctx.Image(), nil
}

// RenderToPath converts the specified PDF page into an image and saves the
// result at the specified location.
func (d *ImageDevice) RenderToPath(page *model.PdfPage, outputPath string) error {
	image, err := d.Render(page)
	if err != nil {
		return err
	}

	extension := strings.ToLower(filepath.Ext(outputPath))
	if extension == "" {
		return errors.New("could not recognize output file type")
	}

	switch extension {
	case ".png":
		return savePNG(outputPath, image)
	case ".jpg", ".jpeg":
		return saveJPG(outputPath, image, 100)
	}

	return fmt.Errorf("unrecognized output file type: %s", extension)
}

func savePNG(path string, image image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, image)
}

func saveJPG(path string, image image.Image, quality int) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return jpeg.Encode(file, image, &jpeg.Options{Quality: quality})
}
