package reporting

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
)

type pdfLine struct {
	Text string
	Font string
	Size float64
}

// WritePDF exports a report to a simple, dependency-free PDF.
func WritePDF(path string, doc ReportDocument, detailed bool) error {
	if err := ensureOutputDir(path); err != nil {
		return err
	}

	pdfBytes, err := renderPDF(doc, detailed)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, pdfBytes, 0o644); err != nil {
		return fmt.Errorf("write pdf: %w", err)
	}
	return nil
}

func renderPDF(doc ReportDocument, detailed bool) ([]byte, error) {
	lines := buildPDFLines(doc, detailed)
	pages := paginatePDFLines(lines)
	pageStreams := make([]string, 0, len(pages))
	for i, page := range pages {
		pageStreams = append(pageStreams, renderPDFPage(page, i+1, len(pages)))
	}
	return assemblePDF(pageStreams), nil
}

func buildPDFLines(doc ReportDocument, detailed bool) []pdfLine {
	lines := []pdfLine{
		{Text: "LLM Cost Guardian Chargeback Report", Font: "F2", Size: 18},
		{Text: fmt.Sprintf("Period: %s (%s to %s)", strings.ToUpper(doc.Period), doc.Start.Format("2006-01-02"), doc.End.Format("2006-01-02")), Font: "F1", Size: 10},
		{Text: fmt.Sprintf("Filters: provider=%s model=%s project=%s", fallback(doc.Filter.Provider, "*"), fallback(doc.Filter.Model, "*"), fallback(doc.Filter.Project, "*")), Font: "F1", Size: 10},
		{Text: "", Font: "F1", Size: 8},
		{Text: "Summary", Font: "F2", Size: 13},
		{Text: fmt.Sprintf("Total cost: $%.4f", doc.Summary.TotalCostUSD), Font: "F1", Size: 10},
		{Text: fmt.Sprintf("Requests: %d", doc.Summary.RecordCount), Font: "F1", Size: 10},
		{Text: fmt.Sprintf("Input tokens: %d", doc.Summary.TotalInputTokens), Font: "F1", Size: 10},
		{Text: fmt.Sprintf("Output tokens: %d", doc.Summary.TotalOutputTokens), Font: "F1", Size: 10},
		{Text: "", Font: "F1", Size: 8},
	}

	lines = append(lines, renderMapSection("Cost by provider", doc.Summary.ByProvider)...)
	lines = append(lines, renderMapSection("Cost by model", doc.Summary.ByModel)...)
	lines = append(lines, renderChargebackSection(doc.Chargebacks)...)

	if detailed {
		lines = append(lines, pdfLine{Text: "Detailed records", Font: "F2", Size: 13})
		lines = append(lines, pdfLine{Text: "TIMESTAMP            PROJECT          PROVIDER        MODEL                          IN       OUT       COST", Font: "F2", Size: 9})
		for _, record := range doc.Records {
			line := fmt.Sprintf(
				"%-20s %-16s %-14s %-30s %8d %8d %10.6f",
				record.Timestamp.Format("2006-01-02 15:04"),
				trimWidth(record.Project, 16),
				trimWidth(record.Provider, 14),
				trimWidth(record.Model, 30),
				record.InputTokens,
				record.OutputTokens,
				record.CostUSD,
			)
			lines = append(lines, pdfLine{Text: line, Font: "F1", Size: 9})
		}
	}

	return lines
}

func renderMapSection(title string, values map[string]float64) []pdfLine {
	lines := []pdfLine{
		{Text: title, Font: "F2", Size: 13},
	}
	if len(values) == 0 {
		return append(lines, pdfLine{Text: "No data", Font: "F1", Size: 10}, pdfLine{Text: "", Font: "F1", Size: 8})
	}

	type kv struct {
		Key   string
		Value float64
	}
	rows := make([]kv, 0, len(values))
	for key, value := range values {
		rows = append(rows, kv{Key: key, Value: value})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Value != rows[j].Value {
			return rows[i].Value > rows[j].Value
		}
		return rows[i].Key < rows[j].Key
	})

	for _, row := range rows {
		lines = append(lines, pdfLine{Text: fmt.Sprintf("%-30s $%10.4f", trimWidth(row.Key, 30), row.Value), Font: "F1", Size: 10})
	}
	lines = append(lines, pdfLine{Text: "", Font: "F1", Size: 8})
	return lines
}

func renderChargebackSection(chargebacks []ProjectChargeback) []pdfLine {
	lines := []pdfLine{
		{Text: "Chargeback by project", Font: "F2", Size: 13},
		{Text: "PROJECT              REQUESTS      INPUT       OUTPUT        COST", Font: "F2", Size: 9},
	}
	if len(chargebacks) == 0 {
		return append(lines, pdfLine{Text: "No project data", Font: "F1", Size: 10}, pdfLine{Text: "", Font: "F1", Size: 8})
	}
	for _, row := range chargebacks {
		lines = append(lines, pdfLine{
			Text: fmt.Sprintf(
				"%-20s %9d %10d %12d %11.4f",
				trimWidth(row.Project, 20),
				row.RecordCount,
				row.TotalInputTokens,
				row.TotalOutputTokens,
				row.TotalCostUSD,
			),
			Font: "F1",
			Size: 10,
		})
	}
	lines = append(lines, pdfLine{Text: "", Font: "F1", Size: 8})
	return lines
}

func paginatePDFLines(lines []pdfLine) [][]pdfLine {
	const usableHeight = 690.0
	pages := make([][]pdfLine, 0, 1)
	current := make([]pdfLine, 0, 64)
	used := 0.0

	for _, line := range lines {
		for _, wrapped := range wrapPDFLine(line) {
			lineHeight := wrapped.Size + 4
			if used+lineHeight > usableHeight && len(current) > 0 {
				pages = append(pages, current)
				current = make([]pdfLine, 0, 64)
				used = 0
			}
			current = append(current, wrapped)
			used += lineHeight
		}
	}

	if len(current) > 0 {
		pages = append(pages, current)
	}
	if len(pages) == 0 {
		pages = append(pages, []pdfLine{{Text: "No data", Font: "F1", Size: 10}})
	}
	return pages
}

func wrapPDFLine(line pdfLine) []pdfLine {
	maxChars := 92
	if line.Size >= 13 {
		maxChars = 72
	}

	if len(line.Text) <= maxChars {
		return []pdfLine{line}
	}

	words := strings.Fields(line.Text)
	if len(words) == 0 {
		return []pdfLine{line}
	}

	lines := make([]pdfLine, 0, 4)
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if len(candidate) <= maxChars {
			current = candidate
			continue
		}
		lines = append(lines, pdfLine{Text: current, Font: line.Font, Size: line.Size})
		current = word
	}
	lines = append(lines, pdfLine{Text: current, Font: line.Font, Size: line.Size})
	return lines
}

func renderPDFPage(lines []pdfLine, pageNumber, pageCount int) string {
	var builder strings.Builder
	y := 760.0

	for _, line := range lines {
		builder.WriteString("BT\n")
		builder.WriteString(fmt.Sprintf("/%s %.0f Tf\n", line.Font, line.Size))
		builder.WriteString(fmt.Sprintf("1 0 0 1 50 %.2f Tm\n", y))
		builder.WriteString(fmt.Sprintf("(%s) Tj\n", escapePDFText(line.Text)))
		builder.WriteString("ET\n")
		y -= line.Size + 4
	}

	builder.WriteString("BT\n")
	builder.WriteString("/F1 9 Tf\n")
	builder.WriteString(fmt.Sprintf("1 0 0 1 50 30 Tm\n(Page %d of %d) Tj\n", pageNumber, pageCount))
	builder.WriteString("ET\n")
	return builder.String()
}

func assemblePDF(pageStreams []string) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Courier >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Courier-Bold >>",
	}

	kids := make([]string, 0, len(pageStreams))
	for i, stream := range pageStreams {
		pageObject := 5 + i*2
		contentObject := pageObject + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", pageObject))
		objects = append(objects,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>", contentObject),
			streamObject(stream),
		)
	}

	objects[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pageStreams))

	var output bytes.Buffer
	output.WriteString("%PDF-1.4\n")

	offsets := make([]int, len(objects)+1)
	for i, object := range objects {
		offsets[i+1] = output.Len()
		output.WriteString(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", i+1, object))
	}

	xrefOffset := output.Len()
	output.WriteString(fmt.Sprintf("xref\n0 %d\n", len(objects)+1))
	output.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objects); i++ {
		output.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}

	output.WriteString(fmt.Sprintf("trailer << /Size %d /Root 1 0 R >>\n", len(objects)+1))
	output.WriteString(fmt.Sprintf("startxref\n%d\n%%%%EOF", xrefOffset))
	return output.Bytes()
}

func streamObject(content string) string {
	return fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)
}

func escapePDFText(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`)
	safe := replacer.Replace(text)
	return strings.Map(func(r rune) rune {
		if r < 32 || r > 126 {
			return '?'
		}
		return r
	}, safe)
}

func trimWidth(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
