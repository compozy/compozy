package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SpacingIssue struct {
	File        string
	Function    string
	LineNumber  int
	IssueType   string
	Description string
}

type FunctionIssues struct {
	File        string
	Function    string
	IssueType   string
	LineNumbers []int
	BlankLines  int
}

var errFuncSpacingViolations = errors.New("function spacing violations detected")

func runFuncSpacingCheck(root string, fix bool) error {
	issues, err := analyzeFunctionSpacing(root)
	if err != nil {
		return err
	}
	if fix {
		fixSpacingIssues(issues)
		return nil
	}
	violations := reportSpacingResults(issues)
	if violations > 0 {
		return fmt.Errorf("%w: %d blank-line issues detected", errFuncSpacingViolations, violations)
	}
	return nil
}

func analyzeFunctionSpacing(root string) ([]SpacingIssue, error) {
	var issues []SpacingIssue
	excluded := strings.Split(excludedDirs, ",")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if shouldSkipDir(info.Name(), excluded) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fileIssues, err := analyzeFileSpacing(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", path, err)
			return nil
		}
		issues = append(issues, fileIssues...)
		return nil
	})
	return issues, err
}

func shouldSkipDir(name string, excluded []string) bool {
	for _, dir := range excluded {
		if name == dir {
			return true
		}
	}
	return false
}

func analyzeFileSpacing(filename string) ([]SpacingIssue, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	lines := strings.Split(string(content), "\n")
	var issues []SpacingIssue
	ast.Inspect(node, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		funcIssues := analyzeFunctionBody(filename, fset, funcDecl, lines)
		issues = append(issues, funcIssues...)
		return true
	})
	return issues, nil
}

func analyzeFunctionBody(filename string, fset *token.FileSet, funcDecl *ast.FuncDecl, lines []string) []SpacingIssue {
	var issues []SpacingIssue
	if funcDecl.Body == nil {
		return issues
	}
	funcName := funcDecl.Name.Name
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		recvType := extractReceiverType(funcDecl.Recv.List[0].Type)
		funcName = fmt.Sprintf("(%s).%s", recvType, funcName)
	}
	stmts := funcDecl.Body.List
	if len(stmts) < 2 {
		return issues
	}
	for i := 0; i < len(stmts)-1; i++ {
		currentStmt := stmts[i]
		nextStmt := stmts[i+1]
		endLine := fset.Position(currentStmt.End()).Line
		startLineNext := fset.Position(nextStmt.Pos()).Line
		actualBlankLines, firstBlankLine := countBlankLines(lines, endLine, startLineNext)
		if actualBlankLines == 0 {
			continue
		}
		issues = append(issues, SpacingIssue{
			File:       filename,
			Function:   funcName,
			LineNumber: firstBlankLine,
			IssueType:  "unnecessary_blank_line",
			Description: fmt.Sprintf(
				"Unnecessary blank line between statements (found %d blank line(s))",
				actualBlankLines,
			),
		})
	}
	return issues
}

func countBlankLines(lines []string, startLine, endLine int) (int, int) {
	if startLine >= endLine-1 {
		return 0, 0
	}
	blankLines := 0
	firstBlankLine := 0
	maxIndex := len(lines)
	for line := startLine + 1; line < endLine; line++ {
		index := line - 1
		if index < 0 || index >= maxIndex {
			continue
		}
		lineContent := strings.TrimSpace(lines[index])
		if lineContent != "" {
			continue
		}
		if firstBlankLine == 0 {
			firstBlankLine = line
		}
		blankLines++
	}
	return blankLines, firstBlankLine
}

func reportSpacingResults(issues []SpacingIssue) int {
	if len(issues) == 0 {
		fmt.Println("✅ All functions have proper spacing!")
		return 0
	}
	groupedIssues := groupIssuesByFunction(issues)
	sort.Slice(groupedIssues, func(i, j int) bool {
		if groupedIssues[i].File == groupedIssues[j].File {
			if groupedIssues[i].Function == groupedIssues[j].Function {
				return groupedIssues[i].LineNumbers[0] < groupedIssues[j].LineNumbers[0]
			}
			return groupedIssues[i].Function < groupedIssues[j].Function
		}
		return groupedIssues[i].File < groupedIssues[j].File
	})
	issueCounts := make(map[string]int)
	totalViolations := 0
	for _, group := range groupedIssues {
		issueCounts[group.IssueType]++
		totalViolations += group.BlankLines
	}
	fmt.Printf(
		"Found %d functions with spacing issues (%d total blank lines) across %d categories:\n\n",
		len(groupedIssues),
		totalViolations,
		len(issueCounts),
	)
	for issueType, count := range issueCounts {
		fmt.Printf("  %s: %d functions\n", issueType, count)
	}
	fmt.Println()
	for _, group := range groupedIssues {
		fmt.Printf("📄 %s\n", group.File)
		fmt.Printf("   Function: %s\n", group.Function)
		lineNumStr := formatLineNumbers(group.LineNumbers)
		fmt.Printf(
			"   Issue: Found %d unnecessary blank line(s) in function body (at lines: %s)\n",
			group.BlankLines,
			lineNumStr,
		)
		fmt.Println()
	}
	fmt.Printf(
		"Total spacing violations: %d blank lines in %d functions\n",
		totalViolations,
		len(groupedIssues),
	)
	return totalViolations
}

func groupIssuesByFunction(issues []SpacingIssue) []FunctionIssues {
	groupMap := make(map[string]*FunctionIssues)
	for _, issue := range issues {
		key := issue.File + "::" + issue.Function
		if _, exists := groupMap[key]; !exists {
			groupMap[key] = &FunctionIssues{
				File:        issue.File,
				Function:    issue.Function,
				IssueType:   issue.IssueType,
				LineNumbers: []int{},
				BlankLines:  0,
			}
		}
		groupMap[key].LineNumbers = append(groupMap[key].LineNumbers, issue.LineNumber)
		blankCount := extractBlankLineCount(issue.Description)
		groupMap[key].BlankLines += blankCount
	}
	var grouped []FunctionIssues
	for _, group := range groupMap {
		sort.Ints(group.LineNumbers)
		grouped = append(grouped, *group)
	}
	return grouped
}

func extractBlankLineCount(description string) int {
	var count int
	//nolint:errcheck // We intentionally ignore parsing errors, defaulting to 1 if parsing fails
	fmt.Sscanf(description, "Unnecessary blank line between statements (found %d blank line(s))", &count)
	if count == 0 {
		return 1
	}
	return count
}

func formatLineNumbers(lines []int) string {
	if len(lines) == 0 {
		return ""
	}
	if len(lines) == 1 {
		return fmt.Sprintf("%d", lines[0])
	}
	if len(lines) <= 5 {
		strs := make([]string, len(lines))
		for i, n := range lines {
			strs[i] = fmt.Sprintf("%d", n)
		}
		return strings.Join(strs, ", ")
	}
	firstFive := make([]string, 5)
	for i := 0; i < 5; i++ {
		firstFive[i] = fmt.Sprintf("%d", lines[i])
	}
	return strings.Join(firstFive, ", ") + fmt.Sprintf(", ... and %d more", len(lines)-5)
}

func fixSpacingIssues(issues []SpacingIssue) {
	if len(issues) == 0 {
		fmt.Println("✅ No spacing issues to fix!")
		return
	}
	fileIssues := groupIssuesByFile(issues)
	fixedFiles := 0
	totalLinesRemoved := 0
	for filePath, fileIssuesList := range fileIssues {
		linesRemoved, err := fixFileSpacing(filePath, fileIssuesList)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fixing %s: %v\n", filePath, err)
			continue
		}
		if linesRemoved > 0 {
			fixedFiles++
			totalLinesRemoved += linesRemoved
			fmt.Printf("✅ Fixed %s: removed %d blank line(s)\n", filePath, linesRemoved)
		}
	}
	fmt.Printf("\n🎉 Fixed %d file(s), removed %d blank line(s) total\n", fixedFiles, totalLinesRemoved)
}

func groupIssuesByFile(issues []SpacingIssue) map[string][]SpacingIssue {
	fileMap := make(map[string][]SpacingIssue)
	for _, issue := range issues {
		fileMap[issue.File] = append(fileMap[issue.File], issue)
	}
	for _, fileIssues := range fileMap {
		sort.Slice(fileIssues, func(i, j int) bool {
			return fileIssues[i].LineNumber < fileIssues[j].LineNumber
		})
	}
	return fileMap
}

func fixFileSpacing(filePath string, issues []SpacingIssue) (int, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("reading file: %w", err)
	}
	lines := bytes.Split(content, []byte("\n"))
	linesToRemove := make(map[int]bool)
	for _, issue := range issues {
		lineIdx := issue.LineNumber - 1
		if lineIdx >= 0 && lineIdx < len(lines) {
			if len(bytes.TrimSpace(lines[lineIdx])) == 0 {
				linesToRemove[lineIdx] = true
			}
		}
	}
	if len(linesToRemove) == 0 {
		return 0, nil
	}
	var newLines [][]byte
	for i, line := range lines {
		if !linesToRemove[i] {
			newLines = append(newLines, line)
		}
	}
	newContent := bytes.Join(newLines, []byte("\n"))
	if err := os.WriteFile(filePath, newContent, 0600); err != nil {
		return 0, fmt.Errorf("writing file: %w", err)
	}
	return len(linesToRemove), nil
}
