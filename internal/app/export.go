package app

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"image"
	imgcolor "image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type exportedMsg struct {
	path string
	err  error
}
type exportRow struct {
	Name       string `json:"name"`
	Agent      string `json:"agent,omitempty"`
	Tokens     any    `json:"tokens"`
	CostUSD    any    `json:"estimated_cost_usd"`
	Cost       any    `json:"estimated_cost"`
	Input      any    `json:"input_tokens"`
	Output     any    `json:"output_tokens"`
	CacheRead  any    `json:"cache_read_tokens"`
	CacheWrite any    `json:"cache_write_tokens"`
}

func jsonMetric(v Metric, rate float64) any {
	if !v.Known {
		return nil
	}
	return map[string]any{"value": v.Value * rate, "partial": v.Partial}
}
func (m model) exportRows() []Row {
	if m.view == 0 {
		return m.chartPeriods()
	}
	return m.rows()
}
func (m model) exportCmd(kind string) tea.Cmd {
	return func() tea.Msg { path, e := m.writeExport(kind); return exportedMsg{path, e} }
}
func (m model) writeExport(kind string) (string, error) {
	rows := m.exportRows()
	var data []byte
	var e error
	switch kind {
	case "json":
		out := []exportRow{}
		for _, r := range rows {
			u := r.Usage
			out = append(out, exportRow{r.Name, r.Agent, jsonMetric(u.Tokens, 1), jsonMetric(u.Cost, 1), jsonMetric(u.Cost, m.fx.Rate), jsonMetric(u.Input, 1), jsonMetric(u.Output, 1), jsonMetric(u.Read, 1), jsonMetric(u.Write, 1)})
		}
		data, e = json.MarshalIndent(map[string]any{"view": views[m.view], "grouping": m.o.Group, "range": m.o.Range, "timezone": m.o.TZ, "agent_filter": m.agent, "model_filter": m.modelFilter, "currency": m.fx.Currency, "exchange_rate": m.fx.Rate, "exchange_date": m.fx.Date, "exchange_source": m.fx.Source, "snapshot_time": m.s.Loaded, "price_source": m.s.PriceSource, "price_date": m.s.PriceDate, "unpriced_models": m.s.Unpriced, "demo": m.o.Demo, "rows": out}, "", "  ")
	case "csv":
		var b bytes.Buffer
		cw := csv.NewWriter(&b)
		_ = cw.Write([]string{"name", "agent", "tokens", "tokens_partial", "estimated_cost_usd", "estimated_cost", "currency", "cost_partial", "input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens", "exchange_rate", "exchange_date", "since", "until", "timezone"})
		metric := func(v Metric, rate float64) string {
			if !v.Known {
				return ""
			}
			return strconv.FormatFloat(v.Value*rate, 'f', -1, 64)
		}
		for _, r := range rows {
			u := r.Usage
			_ = cw.Write([]string{csvSafe(r.Name), csvSafe(r.Agent), metric(u.Tokens, 1), fmt.Sprint(u.Tokens.Partial), metric(u.Cost, 1), metric(u.Cost, m.fx.Rate), m.fx.Currency, fmt.Sprint(u.Cost.Partial), metric(u.Input, 1), metric(u.Output, 1), metric(u.Read, 1), metric(u.Write, 1), fmt.Sprint(m.fx.Rate), m.fx.Date, m.o.Range.Since, m.o.Range.Until, m.o.TZ})
		}
		cw.Flush()
		e = cw.Error()
		data = b.Bytes()
	case "svg":
		data = []byte(m.exportSVG(rows))
	case "png":
		data, e = m.exportPNG(rows)
	default:
		return "", fmt.Errorf("unknown export format")
	}
	if e != nil {
		return "", e
	}
	dir := m.o.ExportDir
	if dir == "" {
		dir = "exports"
	}
	dir, e = filepath.Abs(dir)
	if e != nil {
		return "", e
	}
	if e = os.MkdirAll(dir, 0755); e != nil {
		return "", e
	}
	path := filepath.Join(dir, "tokenlens-"+time.Now().UTC().Format("20060102-150405.000000000")+"."+kind)
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return "", e
	}
	_, e = f.Write(data)
	closeErr := f.Close()
	if e != nil {
		return "", e
	}
	return path, closeErr
}

// Labels remain text when a CSV is opened by a spreadsheet application.
func csvSafe(s string) string {
	if strings.HasPrefix(s, "=") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "@") || strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") {
		return "'" + s
	}
	return s
}
func (m model) chartExportRows(rows []Row) ([]Row, float64) {
	rows = append([]Row(nil), rows...)
	if len(rows) > 30 {
		rows = rows[:30]
	}
	peak := 0.0
	for _, r := range rows {
		peak = max(peak, m.value(r).Value)
	}
	return rows, peak
}
func (m model) exportSubtitle() string {
	return m.o.Range.String() + " · " + m.o.TZ + " · " + m.fx.Currency + " · agent: " + emptyAll(m.agent) + " · model: " + emptyAll(m.modelFilter)
}
func emptyAll(s string) string {
	if s == "" {
		return "all"
	}
	return safe(s)
}
func (m model) exportSVG(rows []Row) string {
	rows, peak := m.chartExportRows(rows)
	height := 160 + len(rows)*44
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="1100" height="%d" viewBox="0 0 1100 %d"><rect width="100%%" height="100%%" fill="#151d28"/><g font-family="monospace" fill="#e3e9f3"><text x="30" y="35" font-size="22">Tokenlens · %s</text><text x="30" y="62" font-size="12">%s</text>`, height, height, html.EscapeString(views[m.view]), html.EscapeString(m.exportSubtitle()))
	for i, r := range rows {
		y := 100 + i*44
		v := m.value(r)
		width := 0.0
		if peak > 0 {
			width = v.Value / peak * 490
		}
		fmt.Fprintf(&b, `<text x="30" y="%d" font-size="12">%s</text><rect x="360" y="%d" width="%.2f" height="15" rx="3" fill="%s"/><text x="870" y="%d" font-size="12">%s</text>`, y, html.EscapeString(clip(safe(r.Name), 42)), y-12, width, string(colorFor(r.Name, m.o.Theme)), y, html.EscapeString(m.formatMetric(v, m.cost && m.view != 3)))
	}
	note := m.fx.label()
	if m.o.Demo {
		note += " · SYNTHETIC DEMO"
	}
	note += " · Chart shows up to 30 rows; CSV/JSON include all filtered rows."
	fmt.Fprintf(&b, `<text x="30" y="%d" font-size="11">%s</text></g></svg>`, height-25, html.EscapeString(note))
	return b.String()
}
func (m model) exportPNG(rows []Row) ([]byte, error) {
	rows, peak := m.chartExportRows(rows)
	height := 160 + len(rows)*44
	img := image.NewRGBA(image.Rect(0, 0, 1100, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{imgcolor.RGBA{21, 29, 40, 255}}, image.Point{}, draw.Src)
	label := func(x, y int, s string) {
		s = strings.NewReplacer("€", "EUR ", "£", "GBP ", "¥", "JPY ", "→", "to", "·", "|").Replace(s)
		d := font.Drawer{Dst: img, Src: &image.Uniform{imgcolor.RGBA{227, 233, 243, 255}}, Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
		d.DrawString(s)
	}
	label(30, 35, "TOKENLENS | "+views[m.view])
	label(30, 62, m.exportSubtitle())
	for i, r := range rows {
		y := 100 + i*44
		v := m.value(r)
		width := 0
		if peak > 0 {
			width = int(v.Value / peak * 490)
		}
		label(30, y, clip(safe(r.Name), 42))
		draw.Draw(img, image.Rect(360, y-12, 360+width, y+3), &image.Uniform{imgcolor.RGBA{128, 216, 195, 255}}, image.Point{}, draw.Src)
		label(870, y, m.formatMetric(v, m.cost && m.view != 3))
	}
	note := m.fx.label()
	if m.o.Demo {
		note += " | SYNTHETIC DEMO"
	}
	label(30, height-40, note)
	label(30, height-20, "Up to 30 chart rows; CSV/JSON include all filtered rows.")
	var b bytes.Buffer
	e := png.Encode(&b, img)
	return b.Bytes(), e
}
