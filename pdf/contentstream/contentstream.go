/*
 * This file is subject to the terms and conditions defined in
 * file 'LICENSE.md', which is part of this source code package.
 */

package contentstream

import (
	"bytes"
	"fmt"

	. "github.com/unidoc/unidoc/pdf/core"
)

type ContentStreamOperation struct {
	Params  []PdfObject
	Operand string
}

type ContentStreamOperations []*ContentStreamOperation

// Convert a set of content stream operations to a content stream byte presentation, i.e. the kind that can be
// stored as a PDF stream or string format.
func (this ContentStreamOperations) Bytes() []byte {
	var buf bytes.Buffer

	for _, op := range this {
		if op == nil {
			continue
		}

		if op.Operand == "BI" {
			// Inline image requires special handling.
			buf.WriteString(op.Operand + "\n")
			buf.WriteString(op.Params[0].DefaultWriteString())

		} else {
			// Default handler.
			for _, param := range op.Params {
				buf.WriteString(param.DefaultWriteString())
				buf.WriteString(" ")

			}

			buf.WriteString(op.Operand + "\n")
		}
	}

	return buf.Bytes()
}

// Parses and extracts all text data in content streams and returns as a string.
// Does not take into account Encoding table, the output is simply the character codes.
func (this *ContentStreamParser) ExtractText() (string, error) {
	operations, err := this.Parse()
	if err != nil {
		return "", err
	}
	inText := false
	txt := ""
	for _, op := range operations {
		if op.Operand == "BT" {
			inText = true
		} else if op.Operand == "ET" {
			inText = false
		}
		if op.Operand == "Td" || op.Operand == "TD" || op.Operand == "T*" {
			// Move to next line...
			txt += "\n"
		}
		if inText && op.Operand == "TJ" {
			if len(op.Params) < 1 {
				continue
			}
			paramList, ok := op.Params[0].(*PdfObjectArray)
			if !ok {
				return "", fmt.Errorf("Invalid parameter type, no array (%T)", op.Params[0])
			}
			for _, obj := range *paramList {
				if strObj, ok := obj.(*PdfObjectString); ok {
					txt += string(*strObj)
				}
			}
		} else if inText && op.Operand == "Tj" {
			if len(op.Params) < 1 {
				continue
			}
			param, ok := op.Params[0].(*PdfObjectString)
			if !ok {
				return "", fmt.Errorf("Invalid parameter type, not string (%T)", op.Params[0])
			}
			txt += string(*param)
		}
	}

	return txt, nil
}
