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

type CommentIssue struct {
	File      string
	Function  string
	StartLine int
	EndLine   int
	Snippet   string
}

var errFuncCommentViolations = errors.New("function comment violations detected")

func runFuncCommentCleanup(root string, fix bool) error {
	issues, err := analyzeFunctionComments(root)
	if err != nil {
		return err
	}
	if fix {
		fixCommentIssues(issues)
		return nil
	}
	violations := reportCommentResults(issues)
	if violations > 0 {
		return fmt.Errorf("%w: %d removable comment block(s) found", errFuncCommentViolations, violations)
	}
	return nil
}

func analyzeFunctionComments(root string) ([]CommentIssue, error) {
	var issues []CommentIssue
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
		fileIssues, err := analyzeFileComments(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", path, err)
			return nil
		}
		issues = append(issues, fileIssues...)
		return nil
	})
	return issues, err
}

func analyzeFileComments(filename string) ([]CommentIssue, error) {
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
	var issues []CommentIssue
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		funcIssues := collectFunctionCommentIssues(filename, fset, funcDecl, node.Comments, lines)
		issues = append(issues, funcIssues...)
	}
	return issues, nil
}

func collectFunctionCommentIssues(
	filename string,
	fset *token.FileSet,
	funcDecl *ast.FuncDecl,
	comments []*ast.CommentGroup,
	lines []string,
) []CommentIssue {
	if funcDecl.Body == nil {
		return nil
	}
	body := funcDecl.Body
	funcName := funcDecl.Name.Name
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
		recvType := extractReceiverType(funcDecl.Recv.List[0].Type)
		funcName = fmt.Sprintf("(%s).%s", recvType, funcName)
	}
	var issues []CommentIssue
	for _, group := range comments {
		if group.Pos() < body.Lbrace || group.Pos() > body.Rbrace {
			continue
		}
		for _, comment := range group.List {
			if isTodoComment(comment.Text) {
				continue
			}
			startLine := fset.Position(comment.Slash).Line
			endLine := fset.Position(comment.End()).Line
			if !isStandaloneComment(lines, startLine, endLine) {
				continue
			}
			snippet := summarizeComment(comment.Text)
			issues = append(issues, CommentIssue{
				File:      filename,
				Function:  funcName,
				StartLine: startLine,
				EndLine:   endLine,
				Snippet:   snippet,
			})
		}
	}
	return issues
}

func isTodoComment(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(trimmed, "// TODO")
}

func isStandaloneComment(lines []string, startLine, endLine int) bool {
	if startLine < 1 || endLine < startLine {
		return false
	}
	totalLines := len(lines)
	for line := startLine; line <= endLine; line++ {
		idx := line - 1
		if idx < 0 || idx >= totalLines {
			return false
		}
		trimmed := strings.TrimSpace(lines[idx])
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "/*") ||
			strings.HasPrefix(trimmed, "*") ||
			strings.HasPrefix(trimmed, "*/") {
			continue
		}
		return false
	}
	return true
}

func summarizeComment(text string) string {
	trimmed := strings.TrimSpace(text)
	trimmed = strings.TrimPrefix(trimmed, "//")
	trimmed = strings.TrimPrefix(trimmed, "/*")
	trimmed = strings.TrimSuffix(trimmed, "*/")
	trimmed = strings.TrimSpace(trimmed)
	replacer := strings.NewReplacer("\n", " ", "\t", " ")
	clean := replacer.Replace(trimmed)
	clean = strings.Join(strings.Fields(clean), " ")
	if clean == "" {
		return "(empty comment)"
	}
	if len(clean) > 70 {
		return clean[:67] + "..."
	}
	return clean
}

func reportCommentResults(issues []CommentIssue) int {
	if len(issues) == 0 {
		fmt.Println("✅ No removable comments found inside function bodies!")
		return 0
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].File == issues[j].File {
			return issues[i].StartLine < issues[j].StartLine
		}
		return issues[i].File < issues[j].File
	})
	fmt.Printf("Found %d removable comment block(s) inside function bodies:\n\n", len(issues))
	for _, issue := range issues {
		lineInfo := fmt.Sprintf("%d", issue.StartLine)
		if issue.EndLine > issue.StartLine {
			lineInfo = fmt.Sprintf("%d-%d", issue.StartLine, issue.EndLine)
		}
		fmt.Printf("📄 %s:%s\n", issue.File, lineInfo)
		fmt.Printf("   Function: %s\n", issue.Function)
		fmt.Printf("   Comment: %s\n\n", issue.Snippet)
	}
	return len(issues)
}

func fixCommentIssues(issues []CommentIssue) {
	if len(issues) == 0 {
		fmt.Println("✅ No comments to remove!")
		return
	}
	fileMap := groupCommentIssuesByFile(issues)
	cleanedFiles := 0
	totalLinesRemoved := 0
	for filePath, fileIssues := range fileMap {
		sort.Slice(fileIssues, func(i, j int) bool {
			if fileIssues[i].StartLine == fileIssues[j].StartLine {
				return fileIssues[i].EndLine < fileIssues[j].EndLine
			}
			return fileIssues[i].StartLine < fileIssues[j].StartLine
		})
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", filePath, err)
			continue
		}
		lines := bytes.Split(content, []byte("\n"))
		linesToRemove := make(map[int]struct{})
		for _, issue := range fileIssues {
			for line := issue.StartLine; line <= issue.EndLine; line++ {
				idx := line - 1
				if idx < 0 || idx >= len(lines) {
					continue
				}
				linesToRemove[idx] = struct{}{}
			}
		}
		if len(linesToRemove) == 0 {
			continue
		}
		var newLines [][]byte
		for idx, line := range lines {
			if _, remove := linesToRemove[idx]; remove {
				totalLinesRemoved++
				continue
			}
			newLines = append(newLines, line)
		}
		if err := os.WriteFile(filePath, bytes.Join(newLines, []byte("\n")), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", filePath, err)
			continue
		}
		cleanedFiles++
		fmt.Printf("✅ Cleaned %s: removed %d comment line(s)\n", filePath, len(linesToRemove))
	}
	fmt.Printf("\n🎉 Cleaned %d file(s), removed %d comment line(s) total\n", cleanedFiles, totalLinesRemoved)
}

func groupCommentIssuesByFile(issues []CommentIssue) map[string][]CommentIssue {
	fileMap := make(map[string][]CommentIssue)
	for _, issue := range issues {
		fileMap[issue.File] = append(fileMap[issue.File], issue)
	}
	return fileMap
}
