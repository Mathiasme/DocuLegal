package models

import (
	"fmt"

	"github.com/unidoc/unioffice/common/license"
	"github.com/unidoc/unioffice/document"

	"github.com/xuri/excelize/v2"
)

func init() {
	// Make sure to load your metered License API key prior to using the library.
	// If you need a key, you can sign up and create a free one at https://cloud.unidoc.io
	err := license.SetMeteredKey(`UNIDOC_KEY`)
	if err != nil {
		panic(err)
	}
}

func ExtractTextFromWordDocument(path string) string {
	doc, err := document.Open(path)
	if err != nil {
		panic(err)
	}
	// To extract the text and work with the formatted info in a simple fashion, you can use:
	extracted := doc.ExtractText()
	for _, e := range extracted.Items {
		//fmt.Println(ei)
		//fmt.Println("Text:", e.Text)
		if e.Run != nil && e.Run.RPr != nil {
			runProps := e.Run.RPr
			//fmt.Println("Bold:", runProps.B != nil)
			//fmt.Println("Italic:", runProps.I != nil)
			if color := runProps.Color; color != nil {
				//fmt.Printf("Color: #%s\n", runProps.Color.ValAttr)
			}
			if highlight := runProps.Highlight; highlight != nil {
				//fmt.Printf("Highlight: %s\n", runProps.Highlight.ValAttr.String())
			}
		}
		if tblInfo := e.TableInfo; tblInfo != nil {
			if tc := tblInfo.Cell; tc != nil {
				//fmt.Println("Row:", tblInfo.RowIndex)
				//fmt.Println("Column:", tblInfo.ColIndex)
				if pr := tc.TcPr; pr != nil {
					if pr.Shd != nil {
						//fmt.Printf("Shade color: #%s\n", pr.Shd.FillAttr)
					}
				}
			}
		}
		if drawingInfo := e.DrawingInfo; drawingInfo != nil {
			//fmt.Println("Height in mm:", measurement.FromEMU(drawingInfo.Height)/measurement.Millimeter)
			//fmt.Println("Width in mm:", measurement.FromEMU(drawingInfo.Width)/measurement.Millimeter)
		}
		//fmt.Println("--------")
	}
	// Alternatively, if just want to work with the flattened text, simply use:
	//fmt.Println("\nFLATTENED:")
	//fmt.Println(extracted.Text())
	return extracted.Text()
}

func ExtractExcelHeader(path string) ([]string, []string) {
		f, err := excelize.OpenFile(path)
		if err != nil {
			fmt.Println(err)
			return nil, nil
		}
		defer func() {
			// Close the spreadsheet.
			if err := f.Close(); err != nil {
				fmt.Println(err)
			}
		}()
		// Get value from cell by given worksheet name and cell reference.
		/*cell, err := f.GetCellValue("Sheet1", "B2")
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(cell)*/
		// Get all the rows in the Sheet1.
		rows, err := f.GetRows("Feuil1")
		if err != nil {
			fmt.Println(err)
			return nil, nil
		}
		/*
		for _, row := range rows {
			for _, colCell := range row {
				fmt.Print(colCell, "\t")
			}
			fmt.Println()
		}*/
		if len(rows) > 1 {
			return rows[0], rows[1]
		}
		return nil, nil
	}
