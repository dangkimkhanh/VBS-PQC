package service

import (
	"fmt"
	"html"
	"html/template"
	"strings"
	"unicode/utf8"
)

// sealColor keeps the stamp visibly different from a red official seal: it
// represents the ML-DSA digital signature, not the institution's legal seal.
const sealColor = "#1e3a8a"

// diplomaSealSVG draws the round digital-signature stamp printed on diplomas:
// "KÝ SỐ HẬU LƯỢNG TỬ" along the top, the university name along the bottom,
// and the ML-DSA parameter set (e.g. "ML-DSA-65") in the centre. It is part of
// the PDF that is hashed and signed, so it cannot be altered without failing
// verification; signing refuses a key whose parameter set differs.
func diplomaSealSVG(universityName, algorithm string) template.HTML {
	name := strings.ToUpper(strings.Join(strings.Fields(universityName), " "))
	if name == "" {
		return ""
	}

	if algorithm == "" {
		algorithm = "ML-DSA"
	}

	// Fit the name between the two stars on the lower arc (about 200 units).
	const arc = 200.0
	runes := float64(utf8.RuneCountInString(name))
	size := arc / (runes * 0.62)
	if size > 15 {
		size = 15
	}
	fit := ""
	if size < 6.5 {
		// Very long names: squeeze the text onto the arc instead of overflowing.
		size = 6.5
		fit = fmt.Sprintf(` textLength="%.0f" lengthAdjust="spacingAndGlyphs"`, arc)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="seal" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200" role="img" aria-label="Dấu chữ ký số">`)
	fmt.Fprintf(&b, `<defs><path id="seal-top" d="M 34,100 A 66,66 0 0 1 166,100"/><path id="seal-bottom" d="M 22,100 A 78,78 0 0 0 178,100"/></defs>`)
	fmt.Fprintf(&b, `<g fill="none" stroke="%s"><circle cx="100" cy="100" r="95" stroke-width="5"/><circle cx="100" cy="100" r="86" stroke-width="1.5"/><circle cx="100" cy="100" r="55" stroke-width="1.5"/></g>`, sealColor)
	fmt.Fprintf(&b, `<g fill="%s" font-family="Arial, Helvetica, sans-serif" font-weight="700" text-anchor="middle">`, sealColor)
	fmt.Fprintf(&b, `<text font-size="14" letter-spacing="1"><textPath href="#seal-top" xlink:href="#seal-top" startOffset="50%%" xmlns:xlink="http://www.w3.org/1999/xlink">KÝ SỐ HẬU LƯỢNG TỬ</textPath></text>`)
	fmt.Fprintf(&b, `<text font-size="%.1f"><textPath href="#seal-bottom" xlink:href="#seal-bottom" startOffset="50%%"%s xmlns:xlink="http://www.w3.org/1999/xlink">%s</textPath></text>`, size, fit, html.EscapeString(name))
	fmt.Fprintf(&b, `<polygon points="%s"/><polygon points="%s"/>`, star(22, 100), star(178, 100))
	fmt.Fprintf(&b, `<text x="100" y="107" font-size="19">%s</text>`, html.EscapeString(algorithm))
	fmt.Fprintf(&b, `</g></svg>`)

	// Built only from fixed markup, the escaped university name and the escaped
	// algorithm label.
	return template.HTML(b.String())
}

// star returns the points of a small five-pointed star centred at (cx, cy).
func star(cx, cy float64) string {
	points := [][2]float64{{0, -6}, {1.8, -1.9}, {5.7, -1.9}, {2.6, 0.8}, {3.6, 5}, {0, 2.6}, {-3.6, 5}, {-2.6, 0.8}, {-5.7, -1.9}, {-1.8, -1.9}}
	parts := make([]string, len(points))
	for i, p := range points {
		parts[i] = fmt.Sprintf("%.1f,%.1f", cx+p[0], cy+p[1])
	}
	return strings.Join(parts, " ")
}
