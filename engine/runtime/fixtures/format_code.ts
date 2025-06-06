// Code formatting tool for testing complex tool operations
export default function run(input: any): Record<string, any> {
  if (!input.code) {
    throw new Error("Code input is required");
  }

  const language = input.language || "unknown";
  const indentSize = input.indent_size || 2;
  const useTabs = input.use_tabs || false;
  const maxLineLength = input.max_line_length || 80;

  const originalLines = input.code.split('\n');
  const changesMade: string[] = [];

  // Simple formatting logic for demo purposes
  const formattedLines = originalLines.map((line: string, index: number) => {
    let formatted = line;

    // Trim whitespace
    const trimmed = line.trim();
    if (trimmed !== line) {
      changesMade.push(`Line ${index + 1}: Trimmed whitespace`);
    }
    formatted = trimmed;

    // Add indentation based on language
    if (language === "javascript" || language === "typescript") {
      // Simple indentation for JS/TS (very basic)
      const openBraces = (formatted.match(/{/g) || []).length;
      if (openBraces > 0 && !formatted.endsWith('{')) {
        const indent = useTabs ? '\t' : ' '.repeat(indentSize);
        formatted = indent + formatted;
        changesMade.push(`Line ${index + 1}: Added indentation`);
      }
    }

    // Check line length
    if (formatted.length > maxLineLength) {
      changesMade.push(`Line ${index + 1}: Exceeds max line length (${formatted.length}/${maxLineLength})`);
    }

    return formatted;
  });

  // Remove empty lines at the end
  while (formattedLines.length > 0 && formattedLines[formattedLines.length - 1].trim() === '') {
    formattedLines.pop();
    changesMade.push("Removed trailing empty lines");
  }

  const formattedCode = formattedLines.join('\n');

  // Return structure matching test expectations
  return {
    formatted_code: formattedCode,
    changes_made: changesMade.join(", "),
    language: language,
    settings: {
      indent_size: indentSize,
      use_tabs: useTabs,
      max_line_length: maxLineLength
    },
    tool_name: "format-code"
  };
}
