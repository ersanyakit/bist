package domain

import "strings"

const (
	AssetTypeUnknown   = 0
	AssetTypeEquity    = 2
	AssetTypeCrypto    = 3
	AssetTypeCommodity = 4
)

const (
	AssetKindEquity    = "equity"
	AssetKindCrypto    = "crypto"
	AssetKindCommodity = "commodity"
)

func IsEquityAssetType(value int) bool {
	return value == AssetTypeEquity
}

func IsEquityOrUnknownAssetType(value int) bool {
	return value == AssetTypeUnknown || value == AssetTypeEquity
}

func IsKnownAssetType(value int) bool {
	switch value {
	case AssetTypeUnknown, AssetTypeEquity, AssetTypeCrypto, AssetTypeCommodity:
		return true
	default:
		return false
	}
}

func NormalizeAssetKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AssetKindCrypto, "coin", "digital_asset", "digital-asset":
		return AssetKindCrypto
	case AssetKindCommodity, "commodities", "metal", "metals", "precious_metal", "precious-metal", "gold", "xau":
		return AssetKindCommodity
	default:
		return AssetKindEquity
	}
}

func IsCryptoAssetKind(value string) bool {
	return NormalizeAssetKind(value) == AssetKindCrypto
}

func IsCommodityAssetKind(value string) bool {
	return NormalizeAssetKind(value) == AssetKindCommodity
}

func AssetKindFromCode(value int) string {
	switch value {
	case AssetTypeCrypto:
		return AssetKindCrypto
	case AssetTypeCommodity:
		return AssetKindCommodity
	default:
		return AssetKindEquity
	}
}

func AssetCodeFromKind(value string) int {
	switch NormalizeAssetKind(value) {
	case AssetKindCrypto:
		return AssetTypeCrypto
	case AssetKindCommodity:
		return AssetTypeCommodity
	default:
		return AssetTypeEquity
	}
}
