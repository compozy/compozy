package main

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const telegramMarkdownV2 = "MarkdownV2"

var (
	telegramFencePattern      = regexp.MustCompile("(?s)```(?:[^\\n]*\\n)?.*?```")
	telegramInlineCodePattern = regexp.MustCompile("`[^`\\n]+`")
	telegramLinkPattern       = regexp.MustCompile(`\[([^\]]+)\]\(([^()]*(?:\([^()]*\)[^()]*)*)\)`)
	telegramHeadingPattern    = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	telegramBoldItalicPattern = regexp.MustCompile(`\*\*\*([^*\n]+)\*\*\*`)
	telegramBoldPattern       = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
	telegramItalicPattern     = regexp.MustCompile(`(?m)(^|[^*])\*(\S(?:[^*\n]*\S)?)\*([^*]|$)`)
	telegramStrikePattern     = regexp.MustCompile(`~~([^~\n]+)~~`)
	telegramQuotePattern      = regexp.MustCompile(`(?m)^(>{1,3}) (.+)$`)
)

type telegramOutboundText struct {
	Text      string
	PlainText string
	ParseMode string
}

func formatOutbound(content string) telegramOutboundText {
	if content == "" || !telegramProtectedRegionsBalanced(content) {
		return telegramOutboundText{Text: content, PlainText: content}
	}

	placeholders := newTelegramPlaceholders(content)
	text := telegramFencePattern.ReplaceAllStringFunc(content, func(value string) string {
		openingEnd := 3
		if newline := strings.Index(value[3:], "\n"); newline >= 0 {
			openingEnd = 3 + newline + 1
		}
		opening := value[:openingEnd]
		body := value[openingEnd : len(value)-3]
		body = strings.NewReplacer("\\", "\\\\", "`", "\\`").Replace(body)
		return placeholders.hold(opening + body + "```")
	})
	text = telegramInlineCodePattern.ReplaceAllStringFunc(text, func(value string) string {
		return placeholders.hold(strings.ReplaceAll(value, "\\", "\\\\"))
	})
	text = telegramLinkPattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := telegramLinkPattern.FindStringSubmatch(value)
		label := escapeTelegramMarkdown(parts[1])
		url := strings.NewReplacer("\\", "\\\\", ")", "\\)").Replace(parts[2])
		return placeholders.hold("[" + label + "](" + url + ")")
	})
	text = telegramHeadingPattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := telegramHeadingPattern.FindStringSubmatch(value)
		inner := strings.TrimSpace(parts[1])
		inner = telegramBoldPattern.ReplaceAllString(inner, "$1")
		return placeholders.hold("*" + escapeTelegramMarkdown(inner) + "*")
	})
	text = telegramBoldItalicPattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := telegramBoldItalicPattern.FindStringSubmatch(value)
		return placeholders.hold("*_" + escapeTelegramMarkdown(parts[1]) + "_*")
	})
	text = telegramBoldPattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := telegramBoldPattern.FindStringSubmatch(value)
		return placeholders.hold("*" + escapeTelegramMarkdown(parts[1]) + "*")
	})
	text = telegramItalicPattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := telegramItalicPattern.FindStringSubmatch(value)
		return parts[1] + placeholders.hold("_"+escapeTelegramMarkdown(parts[2])+"_") + parts[3]
	})
	text = telegramStrikePattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := telegramStrikePattern.FindStringSubmatch(value)
		return placeholders.hold("~" + escapeTelegramMarkdown(parts[1]) + "~")
	})
	text = telegramQuotePattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := telegramQuotePattern.FindStringSubmatch(value)
		return placeholders.hold(parts[1] + " " + escapeTelegramMarkdown(parts[2]))
	})
	text = escapeTelegramMarkdown(text)
	text = placeholders.restore(text)
	return telegramOutboundText{
		Text:      text,
		PlainText: content,
		ParseMode: telegramMarkdownV2,
	}
}

func telegramProtectedRegionsBalanced(value string) bool {
	inFence := false
	inInline := false
	for index := 0; index < len(value); index++ {
		if value[index] == '\\' {
			index++
			continue
		}
		if !inInline && strings.HasPrefix(value[index:], "```") {
			inFence = !inFence
			index += 2
			continue
		}
		if !inFence && value[index] == '`' {
			inInline = !inInline
		}
	}
	return !inFence && !inInline
}

func escapeTelegramMarkdown(value string) string {
	var escaped strings.Builder
	escaped.Grow(len(value))
	for _, current := range value {
		if strings.ContainsRune("_*[]()~`>#+-=|{}.!\\", current) {
			escaped.WriteRune('\\')
		}
		escaped.WriteRune(current)
	}
	return escaped.String()
}

type telegramPlaceholders struct {
	prefix string
	values []string
}

func newTelegramPlaceholders(source string) *telegramPlaceholders {
	prefix := "\x00AGHTELEGRAM"
	for strings.Contains(source, prefix) {
		prefix += "X"
	}
	return &telegramPlaceholders{prefix: prefix}
}

func (p *telegramPlaceholders) hold(value string) string {
	token := p.prefix + strconv.Itoa(len(p.values)) + "\x00"
	p.values = append(p.values, value)
	return token
}

func (p *telegramPlaceholders) restore(value string) string {
	for index, placeholder := range slices.Backward(p.values) {
		token := p.prefix + strconv.Itoa(index) + "\x00"
		value = strings.ReplaceAll(value, token, placeholder)
	}
	return value
}
