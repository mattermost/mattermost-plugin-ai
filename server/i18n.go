package main

import (
	"embed"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed i18n/*.json
var i18nFiles embed.FS

func i18nInit() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.LoadMessageFileFS(i18nFiles, "i18n/es.json")

	return bundle
}
