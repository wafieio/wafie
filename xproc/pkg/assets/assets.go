package assets

import (
	"bytes"
	"embed"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
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

func (a *Assets) RecaptchaPage(data *wv1.ProtectionDesiredState) []byte {
	assetName := "recaptcha.html"
	tmpl, err := template.New(assetName).
		Funcs(sprig.FuncMap()).
		Delims("{{{", "}}}").
		Parse(a.readAssetAsString(assetName))
	if err != nil {
		a.logger.Error(err.Error())
		return nil
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		a.logger.Error(err.Error())
		return nil
	}
	return buf.Bytes()
}

func (a *Assets) readAssetAsString(assetName string) string {
	data, err := assets.ReadFile(assetName)
	if err != nil {
		a.logger.Error("failed to read asset", zap.String("asset_name", assetName), zap.Error(err))
		return ""
	}
	return string(data)
}
