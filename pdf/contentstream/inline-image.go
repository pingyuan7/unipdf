/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package contentstream

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/unidoc/unidoc/common"
	. "github.com/unidoc/unidoc/pdf/core"
	. "github.com/unidoc/unidoc/pdf/model"
)

// A representation of an inline image in a Content stream. Everything between the BI and EI operands.
// ContentStreamInlineImage implements the PdfObject interface although strictly it is not a PDF object.
type ContentStreamInlineImage struct {
	BitsPerComponent PdfObject
	ColorSpace       PdfObject
	Decode           PdfObject
	DecodeParms      PdfObject
	Filter           PdfObject
	Height           PdfObject
	ImageMask        PdfObject
	Intent           PdfObject
	Interpolate      PdfObject
	Width            PdfObject
	stream           []byte
}

func (this *ContentStreamInlineImage) String() string {
	s := fmt.Sprintf("InlineImage(len=%d)\n", len(this.stream))
	if this.BitsPerComponent != nil {
		s += "- BPC " + this.BitsPerComponent.DefaultWriteString() + "\n"
	}
	if this.ColorSpace != nil {
		s += "- CS " + this.ColorSpace.DefaultWriteString() + "\n"
	}
	if this.Decode != nil {
		s += "- D " + this.Decode.DefaultWriteString() + "\n"
	}
	if this.DecodeParms != nil {
		s += "- DP " + this.DecodeParms.DefaultWriteString() + "\n"
	}
	if this.Filter != nil {
		s += "- F " + this.Filter.DefaultWriteString() + "\n"
	}
	if this.Height != nil {
		s += "- H " + this.Height.DefaultWriteString() + "\n"
	}
	if this.ImageMask != nil {
		s += "- IM " + this.ImageMask.DefaultWriteString() + "\n"
	}
	if this.Intent != nil {
		s += "- Intent " + this.Intent.DefaultWriteString() + "\n"
	}
	if this.Interpolate != nil {
		s += "- I " + this.Interpolate.DefaultWriteString() + "\n"
	}
	if this.Width != nil {
		s += "- W " + this.Width.DefaultWriteString() + "\n"
	}
	return s
}

func (this *ContentStreamInlineImage) DefaultWriteString() string {
	var output bytes.Buffer

	// We do not start with "BI" as that is the operand and is written out separately.
	// Write out the parameters
	s := "BPC " + this.BitsPerComponent.DefaultWriteString() + "\n"
	s += "CS " + this.ColorSpace.DefaultWriteString() + "\n"
	s += "D " + this.Decode.DefaultWriteString() + "\n"
	s += "DP " + this.DecodeParms.DefaultWriteString() + "\n"
	s += "F " + this.Filter.DefaultWriteString() + "\n"
	s += "H " + this.Height.DefaultWriteString() + "\n"
	s += "IM " + this.ImageMask.DefaultWriteString() + "\n"
	s += "Intent " + this.Intent.DefaultWriteString() + "\n"
	s += "I " + this.Interpolate.DefaultWriteString() + "\n"
	s += "W " + this.Width.DefaultWriteString() + "\n"
	output.WriteString(s)

	output.WriteString("ID ")
	output.Write(this.stream)

	return output.String()
}

func (this *ContentStreamInlineImage) GetColorSpace(resources *PdfPageResources) (PdfColorspace, error) {
	if this.ColorSpace == nil {
		// Default.
		common.Log.Debug("Inline image not having specified colorspace, assuming Gray")
		return NewPdfColorspaceDeviceGray(), nil
	}

	name, ok := this.ColorSpace.(*PdfObjectName)
	if !ok {
		common.Log.Debug("Error: Invalid object type")
		return nil, errors.New("Invalid type")
	}

	if *name == "G" {
		return NewPdfColorspaceDeviceGray(), nil
	} else if *name == "RGB" {
		return NewPdfColorspaceDeviceRGB(), nil
	} else if *name == "CMYK" {
		return NewPdfColorspaceDeviceCMYK(), nil
	} else if *name == "I" {
		return nil, errors.New("Unsupported Index colorspace")
	} else {
		cs, has := resources.ColorSpace.Colorspaces[string(*name)]
		if !has {
			// Can also refer to a name in the PDF page resources...
			common.Log.Debug("Error, unsupported inline image colorspace: %s", *name)
			return nil, errors.New("Unknown colorspace")
		}

		return cs, nil
	}

}

func (this *ContentStreamInlineImage) GetEncoder() (StreamEncoder, error) {
	return newEncoderFromInlineImage(this)
}

// Export the inline image to Image which can be transformed or exported easily.
// Page resources are needed to look up colorspace information.
func (this *ContentStreamInlineImage) ToImage(resources *PdfPageResources) (*Image, error) {
	// Decode the imaging data if encoded.
	encoder, err := newEncoderFromInlineImage(this)
	if err != nil {
		return nil, err
	}
	common.Log.Trace("encoder: %+v %T", encoder, encoder)

	decoded, err := encoder.DecodeBytes(this.stream)
	if err != nil {
		return nil, err
	}

	image := &Image{}

	// Height.
	if this.Height == nil {
		return nil, errors.New("Height attribute missing")
	}
	height, ok := this.Height.(*PdfObjectInteger)
	if !ok {
		return nil, errors.New("Invalid height")
	}
	image.Height = int64(*height)

	// Width.
	if this.Width == nil {
		return nil, errors.New("Width attribute missing")
	}
	width, ok := this.Width.(*PdfObjectInteger)
	if !ok {
		return nil, errors.New("Invalid width")
	}
	image.Width = int64(*width)

	// BPC.
	if this.BitsPerComponent == nil {
		common.Log.Debug("Inline Bits per component missing - assuming 8")
		image.BitsPerComponent = 8
	} else {
		bpc, ok := this.BitsPerComponent.(*PdfObjectInteger)
		if !ok {
			common.Log.Debug("Error invalid bits per component value, type %T", this.BitsPerComponent)
			return nil, errors.New("BPC Type error")
		}
		image.BitsPerComponent = int64(*bpc)
	}

	// Color components.
	if this.ColorSpace != nil {
		cs, err := this.GetColorSpace(resources)
		if err != nil {
			return nil, err
		}
		image.ColorComponents = cs.GetNumComponents()
	} else {
		// Default gray if not specified.
		common.Log.Debug("Inline Image colorspace not specified - assuming 1 color component")
		image.ColorComponents = 1
	}

	image.Data = decoded

	return image, nil
}

// Parse an inline image from a content stream, both read its properties and binary data.
// When called, "BI" has already been read from the stream.  This function
// finishes reading through "EI" and then returns the ContentStreamInlineImage.
func (this *ContentStreamParser) ParseInlineImage() (*ContentStreamInlineImage, error) {
	// Reading parameters.
	im := ContentStreamInlineImage{}

	for {
		this.skipSpaces()
		obj, err, isOperand := this.parseObject()
		if err != nil {
			return nil, err
		}

		if !isOperand {
			// Not an operand.. Read key value properties..
			param, ok := obj.(*PdfObjectName)
			if !ok {
				return nil, fmt.Errorf("Invalid inline image property (expecting name) - %T", obj)
			}

			valueObj, err, isOperand := this.parseObject()
			if err != nil {
				return nil, err
			}
			if isOperand {
				return nil, fmt.Errorf("Not expecting an operand")
			}

			if *param == "BPC" {
				im.BitsPerComponent = valueObj
			} else if *param == "CS" {
				im.ColorSpace = valueObj
			} else if *param == "D" {
				im.Decode = valueObj
			} else if *param == "DP" {
				im.DecodeParms = valueObj
			} else if *param == "F" {
				im.Filter = valueObj
			} else if *param == "H" {
				im.Height = valueObj
			} else if *param == "IM" {
				im.ImageMask = valueObj
			} else if *param == "Intent" {
				im.Intent = valueObj
			} else if *param == "I" {
				im.Interpolate = valueObj
			} else if *param == "W" {
				im.Width = valueObj
			} else {
				return nil, fmt.Errorf("Unknown inline image parameter %s", *param)
			}
		}

		if isOperand {
			operand, ok := obj.(*PdfObjectString)
			if !ok {
				return nil, fmt.Errorf("Failed to read inline image - invalid operand")
			}

			if *operand == "EI" {
				// Image fully defined
				common.Log.Trace("Inline image finished...")
				return &im, nil
			} else if *operand == "ID" {
				// Inline image data.
				// Should get a single space (0x20) followed by the data and then EI.
				common.Log.Trace("ID start")

				// Skip the space if its there.
				b, err := this.reader.Peek(1)
				if err != nil {
					return nil, err
				}
				if IsWhiteSpace(b[0]) {
					this.reader.Discard(1)
				}

				// Unfortunately there is no good way to know how many bytes to read since it
				// depends on the Filter and encoding etc.
				// Therefore we will simply read until we find "<ws>EI<ws>" where <ws> is whitespace
				// although of course that could be a part of the data (even if unlikely).
				im.stream = []byte{}
				state := 0
				var skipBytes []byte
				for {
					c, err := this.reader.ReadByte()
					if err != nil {
						common.Log.Debug("Unable to find end of image EI in inline image data")
						return nil, err
					}

					if state == 0 {
						if IsWhiteSpace(c) {
							skipBytes = []byte{}
							skipBytes = append(skipBytes, c)
							state = 1
						} else {
							im.stream = append(im.stream, c)
						}
					} else if state == 1 {
						skipBytes = append(skipBytes, c)
						if c == 'E' {
							state = 2
						} else {
							im.stream = append(im.stream, skipBytes...)
							// Need an extra check to decide if we fall back to state 0 or 1.
							if IsWhiteSpace(c) {
								state = 1
							} else {
								state = 0
							}
						}
					} else if state == 2 {
						skipBytes = append(skipBytes, c)
						if c == 'I' {
							state = 3
						} else {
							im.stream = append(im.stream, skipBytes...)
							state = 0
						}
					} else if state == 3 {
						skipBytes = append(skipBytes, c)
						if IsWhiteSpace(c) {
							// image data finished.
							common.Log.Trace("Image stream (%d): % x", len(im.stream), im.stream)
							// Exit point.
							return &im, nil
						} else {
							// Seems like "<ws>EI" was part of the data.
							im.stream = append(im.stream, skipBytes...)
							state = 0
						}
					}
				}
				// Never reached (exit point is at end of EI).
			}
		}
	}
}
