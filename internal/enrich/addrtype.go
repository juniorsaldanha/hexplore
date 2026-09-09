package enrich

import "regexp"

// ClassifyAddress derives the Bitcoin script type purely from the address
// string's own shape — prefix, length and charset give this away without
// needing any chain data. Returns "" for a string that isn't a recognised
// Bitcoin address encoding — notably, Liquid addresses use entirely
// different prefixes and hexplore has no Liquid data source to check one
// against anyway (docs/PLAN.md §12 lists Liquid as unimplemented).
var (
	addrP2TR   = regexp.MustCompile(`^(bc1p|tb1p)[a-z0-9]{58}$`)
	addrP2WSH  = regexp.MustCompile(`^(bc1q|tb1q)[a-z0-9]{58}$`)
	addrP2WPKH = regexp.MustCompile(`^(bc1q|tb1q)[a-z0-9]{38}$`)
	addrP2SH   = regexp.MustCompile(`^(3|2)[a-km-zA-HJ-NP-Z1-9]{25,34}$`)
	addrP2PKH  = regexp.MustCompile(`^(1|m|n)[a-km-zA-HJ-NP-Z1-9]{25,34}$`)
)

func ClassifyAddress(addr string) string {
	switch {
	case addrP2TR.MatchString(addr):
		return "P2TR (Taproot)"
	case addrP2WSH.MatchString(addr):
		return "P2WSH"
	case addrP2WPKH.MatchString(addr):
		return "P2WPKH"
	case addrP2SH.MatchString(addr):
		return "P2SH"
	case addrP2PKH.MatchString(addr):
		return "P2PKH"
	default:
		return ""
	}
}
