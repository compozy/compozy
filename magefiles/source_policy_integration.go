//go:build mage

package main

import (
	"bufio"
	"fmt"
	"go/build/constraint"
	"io"
	"os"
	"strings"
)

func integrationBuildConstraintViolation(path string, rel string) (*sourcePolicyViolation, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open integration test %q: %w", rel, err)
	}
	defer file.Close()

	expr, line, err := readGoBuildConstraint(file)
	if err != nil {
		return nil, fmt.Errorf("read integration build constraint %q: %w", rel, err)
	}
	if expr == nil {
		return &sourcePolicyViolation{
			path:    rel,
			line:    1,
			message: "integration test filename requires a //go:build constraint that implies integration",
		}, nil
	}
	if !constraintRequiresTag(expr, "integration") {
		return &sourcePolicyViolation{
			path:    rel,
			line:    line,
			message: "integration build constraint must imply the integration tag",
		}, nil
	}
	return nil, nil
}

func readGoBuildConstraint(reader io.Reader) (constraint.Expr, int, error) {
	scanner := bufio.NewScanner(reader)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(text, "//go:build ") {
			expr, err := constraint.Parse(text)
			return expr, line, err
		}
		if strings.HasPrefix(text, "package ") {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}
	return nil, 0, nil
}

func constraintRequiresTag(expr constraint.Expr, tag string) bool {
	switch typed := expr.(type) {
	case *constraint.TagExpr:
		return typed.Tag == tag
	case *constraint.NotExpr:
		return false
	case *constraint.AndExpr:
		return constraintRequiresTag(typed.X, tag) || constraintRequiresTag(typed.Y, tag)
	case *constraint.OrExpr:
		return constraintRequiresTag(typed.X, tag) && constraintRequiresTag(typed.Y, tag)
	default:
		return false
	}
}
