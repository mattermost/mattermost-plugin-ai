// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"embed"
	"fmt"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed i18n/*.json
var i18nFiles embed.FS

type TranslationFunc func(translationId string, defaultMessage string, params ...any) string

func i18nInit() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	_, _ = bundle.LoadMessageFileFS(i18nFiles, "i18n/es.json")

	return bundle
}

func i18nLocalizerFunc(bundle *i18n.Bundle, lang string) TranslationFunc {
	localizer := i18n.NewLocalizer(bundle, lang)

	return func(translationId string, defaultMessage string, params ...any) string {
		if len(params) > 0 {
			return fmt.Sprintf(localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    translationId,
					Other: defaultMessage,
				},
			}), params...)
		}
		return localizer.MustLocalize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    translationId,
				Other: defaultMessage,
			},
		})
	}
}
