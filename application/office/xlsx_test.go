package office_test

import (
	"bytes"
	"testing"

	// Packages
	metadata "github.com/mutablelogic/go-metadata"
)

const (
	xlsxType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

	// The sheets: a hidden one, and two in an order which isn't their files'
	xlsxWorkbookXML = `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>
<sheet name="Sales" sheetId="1" r:id="rId2"/>
<sheet name="Secret" sheetId="2" state="hidden" r:id="rId3"/>
<sheet name="Empty" sheetId="3" r:id="rId1"/>
</sheets></workbook>`
	xlsxWorkbookRels = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/>
<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet3.xml"/>
</Relationships>`

	// The text of the cells, one of which is formatted, and one which has
	// its pronunciation
	xlsxSharedStringsXML = `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<si><t>Quarter</t></si>
<si><r><t>Sa</t></r><r><rPr><b/></rPr><t>les</t></r></si>
<si><t>Q1</t><rPh><t>kyu</t></rPh></si>
</sst>`

	// A sheet with a gap between its columns, an empty row, an inline
	// string, a boolean, a number which isn't exact, and a cell without a
	// reference
	xlsxSheetXML = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="C1" t="s"><v>1</v></c></row>
<row r="2"></row>
<row r="3"><c r="A3" t="s"><v>2</v></c><c r="B3" t="inlineStr"><is><r><t>a | b</t></r><rPh><t>ei bi</t></rPh></is></c><c r="C3"><v>0.30000000000000004</v></c><c t="b"><v>1</v></c></row>
</sheetData></worksheet>`

	xlsxEmptySheetXML = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`

	// The workbook's Markdown
	xlsxMarkdown = "## Sheet: Sales\n\n| Quarter |  | Sales |  |\n| --- | --- | --- | --- |\n| Q1 | a \\| b | 0.3 | TRUE |\n\n## Sheet: Empty"
)

func TestXlsxMarkdown(t *testing.T) {
	workbook := document(t, map[string]string{
		"xl/workbook.xml":            xlsxWorkbookXML,
		"xl/_rels/workbook.xml.rels": xlsxWorkbookRels,
		"xl/sharedStrings.xml":       xlsxSharedStringsXML,
		"xl/worksheets/sheet1.xml":   xlsxEmptySheetXML,
		"xl/worksheets/sheet2.xml":   xlsxSheetXML,
		"xl/worksheets/sheet3.xml":   xlsxSheetXML,
	})

	// Each sheet which isn't hidden, in order, with its name, and its rows
	// as a table, its cells in their columns
	meta, err := getMetadata(t.Context(), bytes.NewReader(workbook), xlsxType, metadata.WithNamespace(metadata.MarkdownNamespace))
	require.NoError(t, err)
	require.Len(t, meta, 1)
	assert.Equal(t, xlsxMarkdown, meta[0].Value())

	// A cell whose reference is beyond a sheet's last column isn't read
	sheet := `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>
<row r="1"><c r="A1" t="inlineStr"><is><t>a</t></is></c><c r="B1" t="inlineStr"><is><t>b</t></is></c></row>
<row r="2"><c r="ZZZZZZZZZZZZZZ2" t="inlineStr"><is><t>beyond</t></is></c><c r="A2" t="inlineStr"><is><t>c</t></is></c></row>
</sheetData></worksheet>`
	workbook = document(t, map[string]string{
		"xl/workbook.xml":            `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="S" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": xlsxWorkbookRels,
		"xl/worksheets/sheet1.xml":   sheet,
	})
	meta, err = getMetadata(t.Context(), bytes.NewReader(workbook), xlsxType, metadata.WithNamespace(metadata.MarkdownNamespace))
	require.NoError(t, err)
	require.Len(t, meta, 1)
	assert.Equal(t, "## Sheet: S\n\n| a | b |\n| --- | --- |\n| c |  |", meta[0].Value())
}
