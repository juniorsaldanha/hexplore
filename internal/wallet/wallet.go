// Package wallet derives watch-only addresses from an extended PUBLIC key
// (xpub/ypub/zpub and their testnet counterparts tpub/upub/vpub). This is
// the one place in hexplore that touches BIP32 key material — and it only
// ever touches public keys. An extended private key (xprv/yprv/zprv/...) is
// refused outright: hexplore is read-only by design (see CLAUDE.md), and an
// xpub-family key is exactly what makes watch-only derivation possible in
// the first place — BIP44/49/84/86 all define the account level (m/purpose'
// /coin_type'/account') as the last *hardened* step, specifically so a
// shared account-level public key can still derive every receive/change
// address non-hardened, without ever needing the private key again.
package wallet

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

// ScriptType is which output script a derived address encodes as — the
// convention SLIP-132's version-byte prefix (xpub/ypub/zpub) signals, since
// the underlying BIP32 key math is identical for all of them.
type ScriptType int

const (
	Legacy       ScriptType = iota // BIP44, P2PKH — "1..." / "m.../n..."
	NestedSegWit                   // BIP49, P2SH-wrapped P2WPKH — "3..." / "2..."
	NativeSegWit                   // BIP84, native P2WPKH — "bc1q..." / "tb1q..."
	Taproot                        // BIP86, single-sig key-path-only P2TR — "bc1p..." / "tb1p..."
)

func (s ScriptType) String() string {
	switch s {
	case NestedSegWit:
		return "nested SegWit (P2SH-P2WPKH)"
	case NativeSegWit:
		return "native SegWit (P2WPKH)"
	case Taproot:
		return "Taproot (P2TR)"
	default:
		return "legacy (P2PKH)"
	}
}

// Name is a stable identifier for ScriptType, safe to persist (e.g. in
// cache.WatchedWallet) — unlike String()'s display text, this never
// changes wording across versions.
func (s ScriptType) Name() string {
	switch s {
	case NestedSegWit:
		return "nested-segwit"
	case NativeSegWit:
		return "native-segwit"
	case Taproot:
		return "taproot"
	default:
		return "legacy"
	}
}

// ParseScriptTypeName parses ScriptType.Name()'s output back into a
// ScriptType.
func ParseScriptTypeName(s string) (ScriptType, bool) {
	switch s {
	case "legacy":
		return Legacy, true
	case "nested-segwit":
		return NestedSegWit, true
	case "native-segwit":
		return NativeSegWit, true
	case "taproot":
		return Taproot, true
	default:
		return 0, false
	}
}

// ErrPrivateKey is returned for any extended PRIVATE key. hexplore never
// accepts one, even one pasted by its own rightful owner — see the package
// doc.
var ErrPrivateKey = errors.New("wallet: extended private keys are not supported — hexplore is read-only")

// Standard BIP32 version bytes (what hdkeychain natively understands) plus
// the SLIP-132 variants for BIP49/84 and testnet. A SLIP-132 key carries
// the exact same key material as a plain xpub/tpub — only the version
// bytes differ, purely to signal which derivation convention produced it —
// so parsing swaps them for the standard bytes before handing the key to
// hdkeychain, the well-known trick every wallet supporting them uses.
var versionTable = map[[4]byte]struct {
	scriptType ScriptType
	net        *chaincfg.Params
	private    bool
}{
	{0x04, 0x88, 0xB2, 0x1E}: {Legacy, &chaincfg.MainNetParams, false},        // xpub
	{0x04, 0x88, 0xAD, 0xE4}: {Legacy, &chaincfg.MainNetParams, true},         // xprv
	{0x04, 0x9D, 0x7C, 0xB2}: {NestedSegWit, &chaincfg.MainNetParams, false},  // ypub
	{0x04, 0x9D, 0x78, 0x78}: {NestedSegWit, &chaincfg.MainNetParams, true},   // yprv
	{0x04, 0xB2, 0x47, 0x46}: {NativeSegWit, &chaincfg.MainNetParams, false},  // zpub
	{0x04, 0xB2, 0x43, 0x0C}: {NativeSegWit, &chaincfg.MainNetParams, true},   // zprv
	{0x04, 0x35, 0x87, 0xCF}: {Legacy, &chaincfg.TestNet3Params, false},       // tpub
	{0x04, 0x35, 0x83, 0x94}: {Legacy, &chaincfg.TestNet3Params, true},        // tprv
	{0x04, 0x4A, 0x52, 0x62}: {NestedSegWit, &chaincfg.TestNet3Params, false}, // upub
	{0x04, 0x4A, 0x4E, 0x28}: {NestedSegWit, &chaincfg.TestNet3Params, true},  // uprv
	{0x04, 0x5F, 0x1C, 0xF6}: {NativeSegWit, &chaincfg.TestNet3Params, false}, // vpub
	{0x04, 0x5F, 0x18, 0xBC}: {NativeSegWit, &chaincfg.TestNet3Params, true},  // vprv
}

var standardXpubVersion = []byte{0x04, 0x88, 0xB2, 0x1E}
var standardTpubVersion = []byte{0x04, 0x35, 0x87, 0xCF}

// Key is a parsed, watch-only account-level extended public key, ready to
// derive receive/change addresses.
type Key struct {
	ext        *hdkeychain.ExtendedKey
	ScriptType ScriptType
	Net        *chaincfg.Params
}

// Parse decodes an xpub/ypub/zpub/tpub/upub/vpub into a Key. It verifies
// the key's own checksum and never accepts an extended private key.
func Parse(s string) (*Key, error) {
	raw := base58.Decode(s)
	if len(raw) != 82 {
		return nil, fmt.Errorf("wallet: %q is not a valid extended key", s)
	}
	payload, checksum := raw[:78], raw[78:]
	sum := sha256.Sum256(payload)
	sum = sha256.Sum256(sum[:])
	if !bytes.Equal(sum[:4], checksum) {
		return nil, fmt.Errorf("wallet: %q has an invalid checksum", s)
	}

	var version [4]byte
	copy(version[:], payload[:4])
	info, ok := versionTable[version]
	if !ok {
		return nil, fmt.Errorf("wallet: %q has an unrecognised extended key prefix", s)
	}
	if info.private {
		return nil, ErrPrivateKey
	}

	depth := payload[4]
	parentFP := payload[5:9]
	childNum := binary.BigEndian.Uint32(payload[9:13])
	chainCode := payload[13:45]
	keyData := payload[45:78]

	stdVersion := standardXpubVersion
	if info.net == &chaincfg.TestNet3Params {
		stdVersion = standardTpubVersion
	}
	ext := hdkeychain.NewExtendedKey(stdVersion, keyData, chainCode, parentFP, depth, childNum, false)
	return &Key{ext: ext, ScriptType: info.scriptType, Net: info.net}, nil
}

// Address derives the address at .../chainIdx/index below the account-level
// key k represents — chainIdx 0 is the receive/external chain, 1 is
// change/internal, per BIP44/49/84. Both steps are non-hardened, which is
// exactly what makes deriving them from a public key alone possible.
func (k *Key) Address(chainIdx, index uint32) (string, error) {
	chain, err := k.ext.Derive(chainIdx)
	if err != nil {
		return "", fmt.Errorf("wallet: derive chain %d: %w", chainIdx, err)
	}
	child, err := chain.Derive(index)
	if err != nil {
		return "", fmt.Errorf("wallet: derive index %d: %w", index, err)
	}
	pub, err := child.ECPubKey()
	if err != nil {
		return "", err
	}
	pubKeyHash := btcutil.Hash160(pub.SerializeCompressed())

	switch k.ScriptType {
	case NativeSegWit:
		addr, err := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, k.Net)
		if err != nil {
			return "", err
		}
		return addr.EncodeAddress(), nil
	case NestedSegWit:
		// The P2SH redeem script for a wrapped P2WPKH is simply
		// OP_0 <20-byte-hash> (0x00 0x14 + the hash) — built directly,
		// no script-building helper needed for two fixed bytes.
		redeemScript := append([]byte{0x00, 0x14}, pubKeyHash...)
		scriptHash := btcutil.Hash160(redeemScript)
		addr, err := btcutil.NewAddressScriptHashFromHash(scriptHash, k.Net)
		if err != nil {
			return "", err
		}
		return addr.EncodeAddress(), nil
	case Taproot:
		// Single-sig, key-path-only per BIP86: no script tree, so the
		// tweak (BIP341) is computed over the internal key alone —
		// txscript.ComputeTaprootOutputKey(pub, nil) is exactly that.
		outputKey := txscript.ComputeTaprootOutputKey(pub, nil)
		addr, err := btcutil.NewAddressTaproot(schnorr.SerializePubKey(outputKey), k.Net)
		if err != nil {
			return "", err
		}
		return addr.EncodeAddress(), nil
	default:
		addr, err := btcutil.NewAddressPubKeyHash(pubKeyHash, k.Net)
		if err != nil {
			return "", err
		}
		return addr.EncodeAddress(), nil
	}
}

// AmbiguousScriptType reports whether s's extended-key prefix is
// compatible with more than one derivation convention. A plain xpub/tpub
// is valid for both BIP44 (legacy) and BIP86 (taproot) — they use
// identical version bytes, so the prefix alone can't tell them apart.
// SLIP-132's ypub/zpub and their testnet counterparts are
// convention-specific by construction and are never ambiguous.
//
// Parse defaults an ambiguous key to Legacy; a caller that can ask the
// user which convention they mean should call this first and, if true,
// let the user choose before overriding the parsed Key's ScriptType.
func AmbiguousScriptType(s string) bool {
	raw := base58.Decode(s)
	if len(raw) < 4 {
		return false
	}
	var version [4]byte
	copy(version[:], raw[:4])
	info, ok := versionTable[version]
	return ok && !info.private && info.scriptType == Legacy
}
