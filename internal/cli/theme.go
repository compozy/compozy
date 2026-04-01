package cli

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/compozy/compozy/internal/charmtheme"
)

type cliChromeStyles struct {
	box            lipgloss.Style
	title          lipgloss.Style
	subtitle       lipgloss.Style
	sectionTitle   lipgloss.Style
	label          lipgloss.Style
	value          lipgloss.Style
	separator      lipgloss.Style
	skill          lipgloss.Style
	arrow          lipgloss.Style
	agent          lipgloss.Style
	path           lipgloss.Style
	warn           lipgloss.Style
	successHeader  lipgloss.Style
	successIcon    lipgloss.Style
	failureHeader  lipgloss.Style
	failureIcon    lipgloss.Style
	errorMessage   lipgloss.Style
	formIntro      lipgloss.Style
	formSuccess    lipgloss.Style
	formSuccessSub lipgloss.Style
}

func newCLIChromeStyles() cliChromeStyles {
	return cliChromeStyles{
		box: lipgloss.NewStyle().
			BorderStyle(charmtheme.TechBorder).
			BorderForeground(charmtheme.ColorAccentDeep).
			Background(charmtheme.ColorBgSurface).
			Foreground(charmtheme.ColorFgBright).
			Padding(0, 2).
			MarginBottom(1),
		title:        lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorBrand),
		subtitle:     lipgloss.NewStyle().Foreground(charmtheme.ColorMuted),
		sectionTitle: lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorBrand).MarginTop(1),
		label:        lipgloss.NewStyle().Foreground(charmtheme.ColorMuted),
		value:        lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorAccent),
		separator:    lipgloss.NewStyle().Foreground(charmtheme.ColorBorder),
		skill:        lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorBrand),
		arrow:        lipgloss.NewStyle().Foreground(charmtheme.ColorDim),
		agent:        lipgloss.NewStyle().Foreground(charmtheme.ColorAccentAlt),
		path:         lipgloss.NewStyle().Foreground(charmtheme.ColorMuted),
		warn:         lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorWarning),
		successHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(charmtheme.ColorSuccess).
			MarginTop(1),
		successIcon: lipgloss.NewStyle().Foreground(charmtheme.ColorSuccess),
		failureHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(charmtheme.ColorError).
			MarginTop(1),
		failureIcon: lipgloss.NewStyle().Foreground(charmtheme.ColorError),
		errorMessage: lipgloss.NewStyle().
			Foreground(charmtheme.ColorError),
		formIntro: lipgloss.NewStyle().
			Bold(true).
			Foreground(charmtheme.ColorBrand),
		formSuccess: lipgloss.NewStyle().
			Bold(true).
			Foreground(charmtheme.ColorSuccess),
		formSuccessSub: lipgloss.NewStyle().
			Foreground(charmtheme.ColorMuted),
	}
}

func renderFormIntro() string {
	styles := newCLIChromeStyles()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.formIntro.Render("COMPOZY // INTERACTIVE INPUT"),
		styles.subtitle.Render("Collect parameters for this run"),
	)
	return styles.box.Render(content)
}

func renderFormSuccess() string {
	styles := newCLIChromeStyles()
	return lipgloss.JoinVertical(
		lipgloss.Left,
		styles.formSuccess.Render("OK PARAMETERS COLLECTED"),
		styles.formSuccessSub.Render("Flags are ready to be applied"),
	)
}

func darkHuhTheme() huh.Theme {
	return huh.ThemeFunc(func(bool) *huh.Styles {
		return newDarkHuhStyles()
	})
}

func newDarkHuhStyles() *huh.Styles {
	styles := huh.ThemeBase(true)

	styles.Form.Base = lipgloss.NewStyle().Foreground(charmtheme.ColorFgBright)
	styles.Group.Base = lipgloss.NewStyle().Foreground(charmtheme.ColorFgBright)
	styles.FieldSeparator = lipgloss.NewStyle().SetString("\n\n")
	applyFocusedHuhStyles(styles)
	applyBlurredHuhStyles(styles)
	applyHuhHelpStyles(styles)
	styles.Group.Title = styles.Focused.Title
	styles.Group.Description = styles.Focused.Description
	return styles
}

func applyFocusedHuhStyles(styles *huh.Styles) {
	button := newHuhButtonStyle()
	styles.Focused.Base = lipgloss.NewStyle().
		PaddingLeft(1).
		BorderStyle(charmtheme.TechBorder).
		BorderLeft(true).
		BorderForeground(charmtheme.ColorBorderFocus)
	styles.Focused.Card = styles.Focused.Base
	styles.Focused.Title = lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorBrand)
	styles.Focused.NoteTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(charmtheme.ColorBrand).
		MarginBottom(1)
	styles.Focused.Description = lipgloss.NewStyle().Foreground(charmtheme.ColorMuted)
	styles.Focused.ErrorIndicator = styles.Focused.ErrorIndicator.Foreground(charmtheme.ColorError)
	styles.Focused.ErrorMessage = styles.Focused.ErrorMessage.Foreground(charmtheme.ColorError)
	styles.Focused.SelectSelector = lipgloss.NewStyle().
		Foreground(charmtheme.ColorAccent).
		SetString("> ")
	styles.Focused.NextIndicator = lipgloss.NewStyle().
		MarginLeft(1).
		Foreground(charmtheme.ColorAccent).
		SetString("→")
	styles.Focused.PrevIndicator = lipgloss.NewStyle().
		MarginRight(1).
		Foreground(charmtheme.ColorAccent).
		SetString("←")
	styles.Focused.Option = lipgloss.NewStyle().Foreground(charmtheme.ColorFgBright)
	styles.Focused.Directory = lipgloss.NewStyle().Foreground(charmtheme.ColorAccentAlt)
	styles.Focused.File = lipgloss.NewStyle().Foreground(charmtheme.ColorFgBright)
	styles.Focused.MultiSelectSelector = lipgloss.NewStyle().
		Foreground(charmtheme.ColorAccent).
		SetString("> ")
	styles.Focused.SelectedOption = lipgloss.NewStyle().
		Bold(true).
		Foreground(charmtheme.ColorAccent)
	styles.Focused.SelectedPrefix = lipgloss.NewStyle().
		Foreground(charmtheme.ColorAccent).
		SetString("✓ ")
	styles.Focused.UnselectedOption = lipgloss.NewStyle().Foreground(charmtheme.ColorFgBright)
	styles.Focused.UnselectedPrefix = lipgloss.NewStyle().
		Foreground(charmtheme.ColorDim).
		SetString("• ")
	styles.Focused.FocusedButton = button.
		Foreground(charmtheme.ColorBgBase).
		Background(charmtheme.ColorBrand).
		Bold(true)
	styles.Focused.Next = styles.Focused.FocusedButton
	styles.Focused.BlurredButton = button.
		Foreground(charmtheme.ColorFgBright).
		Background(charmtheme.ColorBgOverlay)
	styles.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(charmtheme.ColorBrand)
	styles.Focused.TextInput.CursorText = lipgloss.NewStyle().
		Foreground(charmtheme.ColorBgBase).
		Background(charmtheme.ColorBrand)
	styles.Focused.TextInput.Placeholder = lipgloss.NewStyle().Foreground(charmtheme.ColorDim)
	styles.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(charmtheme.ColorAccent)
	styles.Focused.TextInput.Text = lipgloss.NewStyle().Foreground(charmtheme.ColorFgBright)
}

func applyBlurredHuhStyles(styles *huh.Styles) {
	styles.Blurred = styles.Focused
	styles.Blurred.Base = lipgloss.NewStyle().
		PaddingLeft(1).
		BorderStyle(lipgloss.HiddenBorder()).
		BorderLeft(true)
	styles.Blurred.Card = styles.Blurred.Base
	styles.Blurred.Title = lipgloss.NewStyle().Bold(true).Foreground(charmtheme.ColorMuted)
	styles.Blurred.NoteTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(charmtheme.ColorMuted).
		MarginBottom(1)
	styles.Blurred.Description = lipgloss.NewStyle().Foreground(charmtheme.ColorDim)
	styles.Blurred.Option = lipgloss.NewStyle().Foreground(charmtheme.ColorMuted)
	styles.Blurred.Directory = lipgloss.NewStyle().Foreground(charmtheme.ColorDim)
	styles.Blurred.File = lipgloss.NewStyle().Foreground(charmtheme.ColorMuted)
	styles.Blurred.SelectSelector = lipgloss.NewStyle().SetString("  ")
	styles.Blurred.MultiSelectSelector = lipgloss.NewStyle().SetString("  ")
	styles.Blurred.SelectedOption = lipgloss.NewStyle().Foreground(charmtheme.ColorMuted)
	styles.Blurred.SelectedPrefix = lipgloss.NewStyle().
		Foreground(charmtheme.ColorDim).
		SetString("✓ ")
	styles.Blurred.UnselectedOption = lipgloss.NewStyle().Foreground(charmtheme.ColorMuted)
	styles.Blurred.UnselectedPrefix = lipgloss.NewStyle().
		Foreground(charmtheme.ColorDim).
		SetString("• ")
	styles.Blurred.NextIndicator = lipgloss.NewStyle()
	styles.Blurred.PrevIndicator = lipgloss.NewStyle()
	styles.Blurred.TextInput.Prompt = lipgloss.NewStyle().Foreground(charmtheme.ColorDim)
	styles.Blurred.TextInput.Placeholder = lipgloss.NewStyle().Foreground(charmtheme.ColorDim)
	styles.Blurred.TextInput.Text = lipgloss.NewStyle().Foreground(charmtheme.ColorMuted)
}

func applyHuhHelpStyles(styles *huh.Styles) {
	styles.Help.Ellipsis = styles.Help.Ellipsis.Foreground(charmtheme.ColorDim)
	styles.Help.ShortKey = styles.Help.ShortKey.Foreground(charmtheme.ColorAccentDeep)
	styles.Help.ShortDesc = styles.Help.ShortDesc.Foreground(charmtheme.ColorMuted)
	styles.Help.ShortSeparator = styles.Help.ShortSeparator.Foreground(charmtheme.ColorDim)
	styles.Help.FullKey = styles.Help.FullKey.Foreground(charmtheme.ColorAccentDeep)
	styles.Help.FullDesc = styles.Help.FullDesc.Foreground(charmtheme.ColorMuted)
	styles.Help.FullSeparator = styles.Help.FullSeparator.Foreground(charmtheme.ColorDim)
}

func newHuhButtonStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Padding(0, 2).
		MarginRight(1)
}
