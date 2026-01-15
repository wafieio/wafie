package assets

import (
	"embed"

	"go.uber.org/zap"
)

// Embed static web assets
//
//go:embed *.html
var assets embed.FS

type Assets struct {
	logger *zap.Logger
}

func New(logger *zap.Logger) *Assets {
	return &Assets{logger: logger}
}

func (a *Assets) BlockPage() []byte {
	assetName := "block.html"
	return []byte(a.readAssetAsString(assetName))
}

func (a *Assets) RecaptchaPage() []byte {
	assetName := "recaptcha.html"
	return []byte(a.readAssetAsString(assetName))
}

func (a *Assets) readAssetAsString(assetName string) string {
	data, err := assets.ReadFile(assetName)
	if err != nil {
		a.logger.Error("failed to read asset", zap.String("asset_name", assetName), zap.Error(err))
		return ""
	}
	return string(data)
}
