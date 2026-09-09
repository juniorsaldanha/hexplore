package components

import "strings"

// bigFont is a 5-row block font covering just what a price tile needs:
// digits, thousands separator, decimal point, and a sign/percent for good
// measure. Anything else renders as a blank cell rather than panicking.
var bigFont = map[rune][5]string{
	'0': {"███", "█ █", "█ █", "█ █", "███"},
	'1': {" █ ", "██ ", " █ ", " █ ", "███"},
	'2': {"███", "  █", "███", "█  ", "███"},
	'3': {"███", "  █", "███", "  █", "███"},
	'4': {"█ █", "█ █", "███", "  █", "  █"},
	'5': {"███", "█  ", "███", "  █", "███"},
	'6': {"███", "█  ", "███", "█ █", "███"},
	'7': {"███", "  █", "  █", "  █", "  █"},
	'8': {"███", "█ █", "███", "█ █", "███"},
	'9': {"███", "█ █", "███", "  █", "███"},
	'.': {"   ", "   ", "   ", "   ", " █ "},
	',': {"   ", "   ", "   ", " █ ", "█  "},
	'+': {"   ", " █ ", "███", " █ ", "   "},
	'-': {"   ", "   ", "███", "   ", "   "},
	'%': {"█ █", "  █", " █ ", "█  ", "█ █"},
	' ': {"   ", "   ", "   ", "   ", "   "},
}

// BigTextHeight is the fixed row count every BigText call returns.
const BigTextHeight = 5

// BigText renders s as 5-row block-letter text, one glyph per rune with a
// single blank column between them. Runes outside bigFont render as a
// blank cell rather than being dropped, so the caller's alignment never
// shifts unexpectedly.
func BigText(s string) []string {
	rows := make([]strings.Builder, BigTextHeight)
	runes := []rune(s)
	for i, r := range runes {
		glyph, ok := bigFont[r]
		if !ok {
			glyph = bigFont[' ']
		}
		for row := range rows {
			rows[row].WriteString(glyph[row])
			if i < len(runes)-1 {
				rows[row].WriteByte(' ')
			}
		}
	}
	out := make([]string, BigTextHeight)
	for i := range rows {
		out[i] = rows[i].String()
	}
	return out
}

// BigTextWidth returns the rendered column width of BigText(s) without
// building it, for layout decisions (e.g. "does this fit the panel").
func BigTextWidth(s string) int {
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	return n*3 + (n - 1)
}
