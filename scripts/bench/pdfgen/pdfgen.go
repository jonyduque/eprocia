// Package pdfgen builds the deterministic synthetic fixtures used by the review
// harness benchmarks: small PDFs that exercise each extraction path of the
// pipeline (native text, scanned image, vector-drawn text, /Contents array),
// an HTML document and a mixed Eproc-style ZIP. All content is fictitious.
package pdfgen

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Page is one page of a generated PDF.
type Page struct {
	Content       string      // content stream operators
	Image         *image.Gray // optional full-page image, drawn by Content as /Im1
	ContentsArray bool        // write /Contents as [ ref ] instead of a single stream
}

// Build serialises objects ("N 0 obj ... endobj") into a PDF with a valid xref.
func Build(objects []string) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = buf.Len()
		buf.WriteString(obj)
		buf.WriteString("\n")
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(objects)+1, xref)
	return buf.Bytes()
}

func stream(num int, dict string, data []byte) string {
	return fmt.Sprintf("%d 0 obj\n<< %s /Length %d >>\nstream\n%s\nendstream\nendobj", num, dict, len(data), data)
}

func deflate(data []byte) []byte {
	var b bytes.Buffer
	w, _ := zlib.NewWriterLevel(&b, zlib.BestCompression)
	w.Write(data)
	w.Close()
	return b.Bytes()
}

// Document assembles pages into an A4 PDF. Object 3 is a shared Helvetica font.
func Document(pages []Page) []byte {
	objs := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj",
		"", // pages tree, filled once kids are known
		"3 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj",
	}
	var kids []string
	for _, p := range pages {
		pageNum := len(objs) + 1
		contentNum := pageNum + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", pageNum))

		res := "/Font << /F1 3 0 R >>"
		if p.Image != nil {
			res += fmt.Sprintf(" /XObject << /Im1 %d 0 R >>", contentNum+1)
		}
		contents := fmt.Sprintf("%d 0 R", contentNum)
		if p.ContentsArray {
			contents = "[ " + contents + " ]"
		}
		objs = append(objs,
			fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << %s >> /Contents %s >>\nendobj", pageNum, res, contents),
			stream(contentNum, "", []byte(p.Content)),
		)
		if p.Image != nil {
			b := p.Image.Bounds()
			objs = append(objs, stream(contentNum+1,
				fmt.Sprintf("/Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceGray /BitsPerComponent 8 /Filter /FlateDecode", b.Dx(), b.Dy()),
				deflate(p.Image.Pix)))
		}
	}
	objs[1] = fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj", strings.Join(kids, " "), len(pages))
	return Build(objs)
}

// Paragraphs of fictitious procedural text (ASCII only: basicfont has no accents).
var paragrafos = []string{
	"PODER JUDICIARIO - JUSTICA FEDERAL - PROCESSO FICTICIO 5000000-00.2026.4.00.0000",
	"Trata-se de acao de procedimento comum ajuizada por Parte Autora Ficticia em face de",
	"Orgao Publico Exemplo, objetivando o reconhecimento de direito descrito na inicial.",
	"A parte autora sustenta, em sintese, que preencheu os requisitos legais exigidos e que",
	"o indeferimento administrativo carece de fundamentacao adequada, razao pela qual requer",
	"a procedencia do pedido, com a condenacao da re ao pagamento das parcelas devidas.",
	"Citada, a parte re apresentou contestacao, alegando preliminarmente a ausencia de interesse",
	"processual e, no merito, a improcedencia dos pedidos formulados na peticao inicial.",
	"E o relatorio. Decido. O feito comporta julgamento antecipado, nos termos do artigo 355.",
	"Ante o exposto, julgo procedente o pedido. Intimem-se. Cumpra-se. Data e assinatura.",
}

// Text returns n lines of fictitious text, cycling through the paragraphs.
func Text(n, seed int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = paragrafos[(i+seed)%len(paragrafos)]
	}
	return lines
}

func escapePDF(s string) string {
	return strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`).Replace(s)
}

// NativeTextPage draws lines as real text (extractable without OCR).
func NativeTextPage(lines []string) Page {
	var b strings.Builder
	b.WriteString("BT /F1 10 Tf 14 TL 50 800 Td\n")
	for _, l := range lines {
		fmt.Fprintf(&b, "(%s) Tj T*\n", escapePDF(l))
	}
	b.WriteString("ET")
	return Page{Content: b.String()}
}

// render rasterises lines with basicfont at the given integer scale.
func render(lines []string, scale int) *image.Gray {
	const lineH = 16
	w, h := 620, 40+lineH*len(lines)
	small := image.NewGray(image.Rect(0, 0, w, h))
	draw.Draw(small, small.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	d := font.Drawer{Dst: small, Src: image.NewUniform(color.Black), Face: basicfont.Face7x13}
	for i, l := range lines {
		d.Dot = fixed.P(10, 24+i*lineH)
		d.DrawString(l)
	}
	if scale == 1 {
		return small
	}
	big := image.NewGray(image.Rect(0, 0, w*scale, h*scale))
	for y := 0; y < h*scale; y++ {
		for x := 0; x < w*scale; x++ {
			big.Pix[y*big.Stride+x] = small.Pix[(y/scale)*small.Stride+x/scale]
		}
	}
	return big
}

// ScannedPage is an image-only page (no text operators), as from a scanner.
func ScannedPage(lines []string) Page {
	img := render(lines, 3)
	b := img.Bounds()
	// Scale the image to the page width keeping its aspect ratio, anchored at the top.
	hPts := 595 * b.Dy() / b.Dx()
	return Page{
		Content: fmt.Sprintf("q 595 0 0 %d 0 %d cm /Im1 Do Q", hPts, 842-hPts),
		Image:   img,
	}
}

// VectorPage draws each dark pixel of the rendered text as a filled rectangle:
// no fonts and no images, like PDFs re-saved by iLovePDF with text as outlines.
func VectorPage(lines []string) Page {
	img := render(lines, 1)
	var b strings.Builder
	b.WriteString("q 0 g\n")
	const px = 0.9 // points per source pixel
	bounds := img.Bounds()
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); {
			if img.Pix[y*img.Stride+x] >= 128 {
				x++
				continue
			}
			start := x
			for x < bounds.Dx() && img.Pix[y*img.Stride+x] < 128 {
				x++
			}
			fmt.Fprintf(&b, "%.1f %.1f %.1f %.1f re\n", 20+float64(start)*px, 820-float64(y)*px, float64(x-start)*px, px)
		}
	}
	b.WriteString("f Q")
	return Page{Content: b.String()}
}

// HTML returns a fictitious Eproc HTML document.
func HTML(lines []string) []byte {
	var b strings.Builder
	b.WriteString("<html><head><title>Evento</title><style>p{margin:0}</style></head><body>\n<h1>CERTIDAO</h1>\n")
	for _, l := range lines {
		fmt.Fprintf(&b, "<p>%s</p>\n", l)
	}
	b.WriteString("<script>var x = 1;</script></body></html>\n")
	return []byte(b.String())
}

// Fixed timestamp keeps the generated ZIP byte-for-byte reproducible.
var zipTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// Entry is one file inside a generated ZIP.
type Entry struct {
	Name string
	Data []byte
}

// Zip writes entries in order with a fixed modification time.
func Zip(entries []Entry) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, _ := zw.CreateHeader(&zip.FileHeader{Name: e.Name, Method: zip.Deflate, Modified: zipTime})
		w.Write(e.Data)
	}
	zw.Close()
	return buf.Bytes()
}

func multiPage(n, seed int) []Page {
	pages := make([]Page, n)
	for i := range pages {
		pages[i] = NativeTextPage(Text(40, seed+i))
	}
	return pages
}

// Fixtures returns every fixture file by name. Names are stable: the
// benchmarks and the fixture test refer to them.
func Fixtures() map[string][]byte {
	scanLines := Text(12, 0)
	vecLines := Text(12, 3)

	files := map[string][]byte{
		"nativo.pdf":         Document(multiPage(3, 0)),
		"escaneado.pdf":      Document([]Page{ScannedPage(scanLines)}),
		"vetorial.pdf":       Document([]Page{VectorPage(vecLines)}),
		"contents-array.pdf": Document([]Page{{Content: NativeTextPage(Text(10, 1)).Content, ContentsArray: true}}),
		"documento.html":     HTML(Text(15, 2)),
	}

	// Mixed ZIP: 25 events with 2 documents each. Most PDFs carry native text;
	// a few are scanned, vector-drawn, /Contents arrays or HTML, as in real downloads.
	var entries []Entry
	for ev := 1; ev <= 25; ev++ {
		for doc := 1; doc <= 2; doc++ {
			n := (ev-1)*2 + doc
			var name string
			var data []byte
			switch {
			case n%10 == 0:
				name, data = fmt.Sprintf("Evento %d - CERT%d.html", ev, doc), HTML(Text(15, n))
			case n%13 == 0:
				name, data = fmt.Sprintf("Evento %d - DOC%d.pdf", ev, doc), files["escaneado.pdf"]
			case n%17 == 0:
				name, data = fmt.Sprintf("Evento %d - OUT%d.pdf", ev, doc), files["vetorial.pdf"]
			case n%23 == 0:
				name, data = fmt.Sprintf("Evento %d - COMP%d.pdf", ev, doc), files["contents-array.pdf"]
			default:
				name, data = fmt.Sprintf("Evento %d - DESPADEC%d.pdf", ev, doc), Document(multiPage(1+n%4, n))
			}
			entries = append(entries, Entry{Name: name, Data: data})
		}
	}
	files["misto.zip"] = Zip(entries)
	return files
}
