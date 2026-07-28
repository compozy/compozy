package bridgeformat

// Case defines one source-markdown expectation for both bridge dialects.
type Case struct {
	Name              string
	Input             string
	Slack             string
	Telegram          string
	TelegramPlain     string
	TelegramParseMode string
}

const (
	markdownV2ParseMode     = "MarkdownV2"
	telegramSpecialsSource  = "v2.0! #tag + x-y = {z} | [q]"
	inlineCodeSource        = "Use `a_b(x)!` now."
	fencedCodeSource        = "```go\nfmt.Println(`x`)\\path\n```"
	slackEntitiesSource     = "Hey <@U123>, see <https://x.y|docs> and <!here>"
	pipeTableSource         = "| A | B |\n|---|---|\n| 1 | 2 |"
	snakeCaseSource         = "🙂 my_variable_name"
	unterminatedFenceSource = "```go\nunterminated **bold**"
)

// Cases returns the shared outbound-markdown behavior corpus.
func Cases() []Case {
	cases := inlineFormattingCases()
	cases = append(cases, fencedCodeCases()...)
	cases = append(cases, platformSyntaxCases()...)
	cases = append(cases, fallbackCases()...)

	return cases
}

func inlineFormattingCases() []Case {
	return []Case{
		{
			Name:              "Should convert bold links and platform control characters",
			Input:             "**bold** [doc](https://x.y) & <tag>",
			Slack:             "*bold* <https://x.y|doc> &amp; &lt;tag&gt;",
			Telegram:          "*bold* [doc](https://x.y) & <tag\\>",
			TelegramPlain:     "**bold** [doc](https://x.y) & <tag>",
			TelegramParseMode: markdownV2ParseMode,
		},
		{
			Name:              "Should convert italic and strikethrough without crossing lines",
			Input:             "*italic* and ~~gone~~\n* list item",
			Slack:             "_italic_ and ~gone~\n* list item",
			Telegram:          "_italic_ and ~gone~\n\\* list item",
			TelegramPlain:     "*italic* and ~~gone~~\n* list item",
			TelegramParseMode: markdownV2ParseMode,
		},
		{
			Name:              "Should escape every Telegram MarkdownV2 special outside code",
			Input:             telegramSpecialsSource,
			Slack:             telegramSpecialsSource,
			Telegram:          "v2\\.0\\! \\#tag \\+ x\\-y \\= \\{z\\} \\| \\[q\\]",
			TelegramPlain:     telegramSpecialsSource,
			TelegramParseMode: markdownV2ParseMode,
		},
		{
			Name:              "Should preserve inline code while escaping surrounding prose",
			Input:             inlineCodeSource,
			Slack:             inlineCodeSource,
			Telegram:          "Use `a_b(x)!` now\\.",
			TelegramPlain:     inlineCodeSource,
			TelegramParseMode: markdownV2ParseMode,
		},
	}
}

func fencedCodeCases() []Case {
	return []Case{
		{
			Name:              "Should preserve fenced code and escape Telegram code delimiters",
			Input:             fencedCodeSource,
			Slack:             fencedCodeSource,
			Telegram:          "```go\nfmt.Println(\\`x\\`)\\\\path\n```",
			TelegramPlain:     fencedCodeSource,
			TelegramParseMode: markdownV2ParseMode,
		},
		{
			Name:              "Should resume conversion after a balanced code fence",
			Input:             "```go\n**literal**\n```\n**bold**",
			Slack:             "```go\n**literal**\n```\n*bold*",
			Telegram:          "```go\n**literal**\n```\n*bold*",
			TelegramPlain:     "```go\n**literal**\n```\n**bold**",
			TelegramParseMode: markdownV2ParseMode,
		},
	}
}

func platformSyntaxCases() []Case {
	return []Case{
		{
			Name:              "Should preserve Slack entities and escape them as Telegram prose",
			Input:             slackEntitiesSource,
			Slack:             slackEntitiesSource,
			Telegram:          "Hey <@U123\\>, see <https://x\\.y\\|docs\\> and <\\!here\\>",
			TelegramPlain:     slackEntitiesSource,
			TelegramParseMode: markdownV2ParseMode,
		},
		{
			Name:              "Should keep pipe tables readable in both dialects",
			Input:             pipeTableSource,
			Slack:             pipeTableSource,
			Telegram:          "\\| A \\| B \\|\n\\|\\-\\-\\-\\|\\-\\-\\-\\|\n\\| 1 \\| 2 \\|",
			TelegramPlain:     pipeTableSource,
			TelegramParseMode: markdownV2ParseMode,
		},
		{
			Name:              "Should preserve astral emoji and snake case identifiers",
			Input:             snakeCaseSource,
			Slack:             snakeCaseSource,
			Telegram:          "🙂 my\\_variable\\_name",
			TelegramPlain:     snakeCaseSource,
			TelegramParseMode: markdownV2ParseMode,
		},
	}
}

func fallbackCases() []Case {
	return []Case{
		{
			Name:              "Should fall back to plain text for an unterminated code fence",
			Input:             unterminatedFenceSource,
			Slack:             unterminatedFenceSource,
			Telegram:          unterminatedFenceSource,
			TelegramPlain:     unterminatedFenceSource,
			TelegramParseMode: "",
		},
	}
}
