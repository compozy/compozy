package main

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	slackFencePattern      = regexp.MustCompile("(?s)```(?:[^\\n]*\\n)?(?:.*?```|.*$)")
	slackInlineCodePattern = regexp.MustCompile("`[^`\\n]+`")
	slackLinkPattern       = regexp.MustCompile(`!?\[([^\]]+)\]\(([^()]*(?:\([^()]*\)[^()]*)*)\)`)
	slackEntityPattern     = regexp.MustCompile(`<(?:[@#!]|(?:https?|mailto|tel):)[^>\n]+>`)
	slackQuotePattern      = regexp.MustCompile(`(?m)^(>+\s)`)
	slackHeadingPattern    = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	slackBoldItalicPattern = regexp.MustCompile(`\*\*\*([^*\n]+)\*\*\*`)
	slackBoldPattern       = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
	slackItalicPattern     = regexp.MustCompile(`(?m)(^|[^*])\*(\S(?:[^*\n]*\S)?)\*([^*]|$)`)
	slackStrikePattern     = regexp.MustCompile(`~~([^~\n]+)~~`)
)

func formatOutbound(content string) string {
	if content == "" {
		return content
	}

	placeholders := newSlackPlaceholders(content)
	text := slackFencePattern.ReplaceAllStringFunc(content, placeholders.hold)
	text = slackInlineCodePattern.ReplaceAllStringFunc(text, placeholders.hold)
	text = slackLinkPattern.ReplaceAllStringFunc(text, func(value string) string {
		if strings.HasPrefix(value, "!") {
			return value
		}
		parts := slackLinkPattern.FindStringSubmatch(value)
		label := escapeSlackText(parts[1])
		url := strings.TrimSpace(parts[2])
		url = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(url, "<"), ">"))
		return placeholders.hold("<" + url + "|" + label + ">")
	})
	text = slackEntityPattern.ReplaceAllStringFunc(text, placeholders.hold)
	text = slackQuotePattern.ReplaceAllStringFunc(text, placeholders.hold)
	text = escapeSlackText(text)
	text = slackHeadingPattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := slackHeadingPattern.FindStringSubmatch(value)
		inner := strings.TrimSpace(parts[1])
		inner = slackBoldPattern.ReplaceAllString(inner, "$1")
		return placeholders.hold("*" + inner + "*")
	})
	text = slackBoldItalicPattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := slackBoldItalicPattern.FindStringSubmatch(value)
		return placeholders.hold("*_" + parts[1] + "_*")
	})
	text = slackBoldPattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := slackBoldPattern.FindStringSubmatch(value)
		return placeholders.hold("*" + parts[1] + "*")
	})
	text = slackItalicPattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := slackItalicPattern.FindStringSubmatch(value)
		return parts[1] + placeholders.hold("_"+parts[2]+"_") + parts[3]
	})
	text = slackStrikePattern.ReplaceAllStringFunc(text, func(value string) string {
		parts := slackStrikePattern.FindStringSubmatch(value)
		return placeholders.hold("~" + parts[1] + "~")
	})
	return placeholders.restore(text)
}

func escapeSlackText(value string) string {
	value = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">").Replace(value)
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(value)
}

type slackPlaceholders struct {
	prefix string
	values []string
}

func newSlackPlaceholders(source string) *slackPlaceholders {
	prefix := "\x00AGHSLACK"
	for strings.Contains(source, prefix) {
		prefix += "X"
	}
	return &slackPlaceholders{prefix: prefix}
}

func (p *slackPlaceholders) hold(value string) string {
	token := p.prefix + strconv.Itoa(len(p.values)) + "\x00"
	p.values = append(p.values, value)
	return token
}

func (p *slackPlaceholders) restore(value string) string {
	for index, placeholder := range slices.Backward(p.values) {
		token := p.prefix + strconv.Itoa(index) + "\x00"
		value = strings.ReplaceAll(value, token, placeholder)
	}
	return value
}
