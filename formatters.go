package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	base := filepath.Base(path)
	hashPath := path + ".sha256"
	content := fmt.Sprintf("%s  %s\n", digest, base)
	if err := os.WriteFile(hashPath, []byte(content), 0o644); err != nil {
		return digest, err
	}
	return digest, nil
}

// writeReport dispatches to the correct format writer.
func writeReport(data *ReportData, dir, baseName, format string) ([]string, string, error) {
	switch strings.ToLower(format) {
	case "csv":
		return writeCSV(data, dir, baseName)
	case "json":
		return writeJSON(data, dir, baseName)
	case "excel":
		return writeExcel(data, dir, baseName)
	case "html":
		return writeHTML(data, dir, baseName)
	default:
		return nil, "", fmt.Errorf("unknown format: %s", format)
	}
}

// --- CSV ---

func writeCSV(data *ReportData, dir, baseName string) ([]string, string, error) {
	path := filepath.Join(dir, baseName+".csv")
	f, err := os.Create(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write(data.Combined.Headers)
	for _, row := range data.Combined.Rows {
		w.Write(row)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}

	digest, err := hashFile(path)
	return []string{filepath.Base(path)}, digest, err
}

// --- JSON ---

func writeJSON(data *ReportData, dir, baseName string) ([]string, string, error) {
	path := filepath.Join(dir, baseName+".json")

	out := map[string]any{
		"metadata": data.Metadata,
	}
	for _, section := range data.Sections {
		rows := make([]map[string]string, 0, len(section.Rows))
		for _, row := range section.Rows {
			m := make(map[string]string)
			for i, h := range section.Headers {
				if i < len(row) {
					m[h] = row[i]
				}
			}
			rows = append(rows, m)
		}
		out["data"] = rows
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, "", err
	}

	digest, err := hashFile(path)
	return []string{filepath.Base(path)}, digest, err
}

// --- Excel ---

func writeExcel(data *ReportData, dir, baseName string) ([]string, string, error) {
	f := excelize.NewFile()
	defer f.Close()

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"2F5496"}, Pattern: 1},
		Border: []excelize.Border{
			{Type: "left", Style: 1, Color: "000000"},
			{Type: "right", Style: 1, Color: "000000"},
			{Type: "top", Style: 1, Color: "000000"},
			{Type: "bottom", Style: 1, Color: "000000"},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", WrapText: true},
	})

	// Metadata sheet
	meta := f.GetSheetName(0)
	f.SetSheetName(meta, "Report Info")
	boldStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	row := 1
	for k, v := range data.Metadata {
		f.SetCellValue("Report Info", fmt.Sprintf("A%d", row), k)
		f.SetCellStyle("Report Info", fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), boldStyle)
		f.SetCellValue("Report Info", fmt.Sprintf("B%d", row), v)
		row++
	}
	f.SetColWidth("Report Info", "A", "A", 25)
	f.SetColWidth("Report Info", "B", "B", 60)

	// Data sheet
	for _, section := range data.Sections {
		sheet, _ := f.NewSheet(section.Title)
		_ = sheet

		for col, h := range section.Headers {
			cell, _ := excelize.CoordinatesToCellName(col+1, 1)
			f.SetCellValue(section.Title, cell, h)
		}
		lastCell, _ := excelize.CoordinatesToCellName(len(section.Headers), 1)
		f.SetCellStyle(section.Title, "A1", lastCell, headerStyle)

		for rowIdx, r := range section.Rows {
			for colIdx, val := range r {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
				f.SetCellValue(section.Title, cell, val)
			}
		}

		for col := range section.Headers {
			maxLen := len(section.Headers[col])
			for _, r := range section.Rows {
				if col < len(r) && len(r[col]) > maxLen {
					maxLen = len(r[col])
				}
			}
			if maxLen > 50 {
				maxLen = 50
			}
			colName, _ := excelize.ColumnNumberToName(col + 1)
			f.SetColWidth(section.Title, colName, colName, float64(maxLen+3))
		}
	}

	path := filepath.Join(dir, baseName+".xlsx")
	if err := f.SaveAs(path); err != nil {
		return nil, "", err
	}
	digest, err := hashFile(path)
	return []string{filepath.Base(path)}, digest, err
}

// --- HTML ---

func writeHTML(data *ReportData, dir, baseName string) ([]string, string, error) {
	path := filepath.Join(dir, baseName+".html")
	var b strings.Builder
	h := html.EscapeString

	reportName := data.Metadata["Report"]
	if reportName == "" {
		reportName = "ConductorOne Report"
	}

	b.WriteString(`<!DOCTYPE html>
<html><head><meta charset="utf-8">
<title>` + h(reportName) + `</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; margin: 2em; color: #333; }
h1 { color: #2F5496; }
h2 { color: #2F5496; margin-top: 2em; border-bottom: 2px solid #2F5496; padding-bottom: 0.3em; }
table { border-collapse: collapse; width: 100%; margin-bottom: 2em; font-size: 0.9em; }
th { background: #2F5496; color: white; padding: 8px 12px; text-align: left; }
td { padding: 6px 12px; border-bottom: 1px solid #ddd; }
tr:hover { background: #f5f5f5; }
.meta-table td:first-child { font-weight: bold; width: 200px; }
.meta-table { width: auto; }
@media print { body { margin: 0.5em; } table { font-size: 0.75em; } }
</style></head><body>
<h1>` + h(reportName) + `</h1>
<h2>Report Info</h2>
<table class="meta-table">
`)
	for k, v := range data.Metadata {
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td></tr>\n", h(k), h(v))
	}
	b.WriteString("</table>\n")

	for _, section := range data.Sections {
		fmt.Fprintf(&b, "<h2>%s</h2>\n<table>\n<thead><tr>", h(section.Title))
		for _, header := range section.Headers {
			fmt.Fprintf(&b, "<th>%s</th>", h(header))
		}
		b.WriteString("</tr></thead>\n<tbody>\n")
		for _, row := range section.Rows {
			b.WriteString("<tr>")
			for _, val := range row {
				fmt.Fprintf(&b, "<td>%s</td>", h(val))
			}
			b.WriteString("</tr>\n")
		}
		b.WriteString("</tbody></table>\n")
	}
	b.WriteString("</body></html>")

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return nil, "", err
	}
	digest, err := hashFile(path)
	return []string{filepath.Base(path)}, digest, err
}
