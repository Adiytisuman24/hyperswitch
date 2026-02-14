package locale

import (
	"embed"
	"fmt"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v3"
)

//go:embed translations/*.yaml
var translationsFS embed.FS

// Locale represents a supported locale
type Locale string

const (
	LocaleEnUS Locale = "en-US"
	LocaleEsES Locale = "es-ES"
	LocaleFrFR Locale = "fr-FR"
	LocaleDeDE Locale = "de-DE"
	LocaleItIT Locale = "it-IT"
	LocaleJaJP Locale = "ja-JP"
	LocaleZhCN Locale = "zh-CN"
	LocaleArSA Locale = "ar-SA" // RTL
	LocaleHiIN Locale = "hi-IN"
	LocalePtBR Locale = "pt-BR"
	LocaleRuRU Locale = "ru-RU"
	LocaleKoKR Locale = "ko-KR"
)

// Service handles localization operations
type Service struct {
	bundle   *i18n.Bundle
	printers map[language.Tag]*message.Printer
}

// NewService creates a new locale service
func NewService() (*Service, error) {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	// Load all translation files
	entries, err := translationsFS.ReadDir("translations")
	if err != nil {
		return nil, fmt.Errorf("failed to read translations directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := translationsFS.ReadFile("translations/" + entry.Name())
		if err != nil {
			continue
		}

		if _, err := bundle.ParseMessageFileBytes(data, entry.Name()); err != nil {
			return nil, fmt.Errorf("failed to parse translation file %s: %w", entry.Name(), err)
		}
	}

	// Create number/currency printers for each supported language
	printers := make(map[language.Tag]*message.Printer)
	for _, locale := range GetSupportedLocales() {
		tag, _ := language.Parse(string(locale))
		printers[tag] = message.NewPrinter(tag)
	}

	return &Service{
		bundle:   bundle,
		printers: printers,
	}, nil
}

// Localizer returns a localizer for the given locale
func (s *Service) GetLocalizer(locale Locale) *i18n.Localizer {
	accept := string(locale)
	return i18n.NewLocalizer(s.bundle, accept)
}

// Translate translates a message key
func (s *Service) Translate(locale Locale, key string, args map[string]interface{}) string {
	localizer := s.GetLocalizer(locale)

	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: args,
	})

	if err != nil {
		return key // Return key if translation not found
	}

	return msg
}

// FormatCurrency formats a currency amount according to locale
func (s *Service) FormatCurrency(locale Locale, amount float64, currency string) string {
	tag, _ := language.Parse(string(locale))
	printer, exists := s.printers[tag]
	if !exists {
		printer = message.NewPrinter(language.English)
	}

	// Format with 2 decimal places
	formatted := printer.Sprintf("%.2f", amount)

	// Add currency symbol based on locale
	symbol := getCurrencySymbol(currency)

	// Position symbol based on locale
	if isRTL(locale) {
		return fmt.Sprintf("%s %s", formatted, symbol)
	}

	return fmt.Sprintf("%s%s", symbol, formatted)
}

// FormatDate formats a date according to locale
func (s *Service) FormatDate(locale Locale, date string, format string) string {
	// Simplified date formatting
	// In production, use time.Time and proper locale-aware formatting
	return date
}

// FormatNumber formats a number according to locale
func (s *Service) FormatNumber(locale Locale, number float64) string {
	tag, _ := language.Parse(string(locale))
	printer, exists := s.printers[tag]
	if !exists {
		printer = message.NewPrinter(language.English)
	}

	return printer.Sprintf("%.2f", number)
}

// GetPreferredPaymentMethods returns payment methods ordered by region preference
func (s *Service) GetPreferredPaymentMethods(locale Locale) []string {
	region := getRegionFromLocale(locale)

	preferences := map[string][]string{
		"NA":    {"card", "apple_pay", "google_pay", "paypal"},
		"EU":    {"card", "sepa", "ideal", "sofort", "klarna"},
		"APAC":  {"card", "alipay", "wechat_pay", "paynow"},
		"LATAM": {"card", "pix", "mercado_pago", "oxxo"},
		"MENA":  {"card", "mada", "saddad", "benefit_pay"},
	}

	if methods, exists := preferences[region]; exists {
		return methods
	}

	return preferences["NA"] // Default
}

// CheckoutContent represents localized checkout content
type CheckoutContent struct {
	Locale             Locale            `json:"locale"`
	Title              string            `json:"title"`
	PayButtonText      string            `json:"pay_button_text"`
	CancelButtonText   string            `json:"cancel_button_text"`
	SecurityNote       string            `json:"security_note"`
	TermsAndConditions string            `json:"terms_and_conditions"`
	ErrorMessages      map[string]string `json:"error_messages"`
	Direction          string            `json:"direction"` // ltr or rtl
}

// GetCheckoutContent returns localized checkout content
func (s *Service) GetCheckoutContent(locale Locale) *CheckoutContent {
	localizer := s.GetLocalizer(locale)

	content := &CheckoutContent{
		Locale:             locale,
		Title:              s.Translate(locale, "checkout.title", nil),
		PayButtonText:      s.Translate(locale, "checkout.pay_button", nil),
		CancelButtonText:   s.Translate(locale, "checkout.cancel_button", nil),
		SecurityNote:       s.Translate(locale, "checkout.security_note", nil),
		TermsAndConditions: s.Translate(locale, "checkout.terms", nil),
		ErrorMessages:      s.getLocalizedErrors(localizer),
		Direction:          "ltr",
	}

	if isRTL(locale) {
		content.Direction = "rtl"
	}

	return content
}

// Helper functions

func (s *Service) getLocalizedErrors(localizer *i18n.Localizer) map[string]string {
	errorKeys := []string{
		"error.invalid_card",
		"error.card_declined",
		"error.insufficient_funds",
		"error.network_error",
		"error.invalid_cvv",
		"error.invalid_expiry",
	}

	errors := make(map[string]string)
	for _, key := range errorKeys {
		msg, _ := localizer.Localize(&i18n.LocalizeConfig{
			MessageID: key,
		})
		errors[key] = msg
	}

	return errors
}

func isRTL(locale Locale) bool {
	rtlLocales := []Locale{LocaleArSA}
	for _, rtl := range rtlLocales {
		if locale == rtl {
			return true
		}
	}
	return false
}

func getRegionFromLocale(locale Locale) string {
	regionMap := map[Locale]string{
		LocaleEnUS: "NA",
		LocaleEsES: "EU",
		LocaleFrFR: "EU",
		LocaleDeDE: "EU",
		LocaleItIT: "EU",
		LocaleJaJP: "APAC",
		LocaleZhCN: "APAC",
		LocaleArSA: "MENA",
		LocaleHiIN: "APAC",
		LocalePtBR: "LATAM",
		LocaleRuRU: "EU",
		LocaleKoKR: "APAC",
	}

	if region, exists := regionMap[locale]; exists {
		return region
	}
	return "NA"
}

func getCurrencySymbol(currency string) string {
	symbols := map[string]string{
		"USD": "$",
		"EUR": "€",
		"GBP": "£",
		"JPY": "¥",
		"INR": "₹",
		"CNY": "¥",
		"BRL": "R$",
		"RUB": "₽",
		"SAR": "﷼",
	}

	if symbol, exists := symbols[currency]; exists {
		return symbol
	}
	return currency
}

// GetSupportedLocales returns all supported locales
func GetSupportedLocales() []Locale {
	return []Locale{
		LocaleEnUS, LocaleEsES, LocaleFrFR, LocaleDeDE, LocaleItIT,
		LocaleJaJP, LocaleZhCN, LocaleArSA, LocaleHiIN, LocalePtBR,
		LocaleRuRU, LocaleKoKR,
	}
}

// ValidateLocale checks if a locale is supported
func ValidateLocale(locale string) bool {
	for _, supported := range GetSupportedLocales() {
		if string(supported) == locale {
			return true
		}
	}
	return false
}
