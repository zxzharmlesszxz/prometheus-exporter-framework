package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"sort"
	"strings"
)

type symbol struct {
	key   string
	order int
	text  string
}

func main() {
	os.Exit(runSymbolDiff(os.Args[1:], os.Stdout, os.Stderr))
}

func runSymbolDiff(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("go-symbol-diff", flag.ContinueOnError)
	flags.SetOutput(stderr)
	leftLabel := flags.String("left-label", "left", "label for the left file")
	rightLabel := flags.String("right-label", "right", "label for the right file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 2 {
		_, _ = fmt.Fprintln(stderr, "usage: go-symbol-diff [--left-label label] [--right-label label] LEFT.go RIGHT.go")
		return 2
	}

	leftPath := flags.Arg(0)
	rightPath := flags.Arg(1)

	left, err := symbols(leftPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "parse %s: %v\n", leftPath, err)
		return 2
	}
	right, err := symbols(rightPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "parse %s: %v\n", rightPath, err)
		return 2
	}

	keys := make(map[string]struct{}, len(left)+len(right))
	for key := range left {
		keys[key] = struct{}{}
	}
	for key := range right {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)

	for _, key := range ordered {
		leftSymbol, leftOK := left[key]
		rightSymbol, rightOK := right[key]
		switch {
		case !leftOK:
			_, _ = fmt.Fprintf(stdout, "SYMBOL MISSING target %s\n", key)
			_, _ = fmt.Fprintf(stdout, "+++ %s %s\n%s\n", *rightLabel, key, indent(rightSymbol.text))
		case !rightOK:
			_, _ = fmt.Fprintf(stdout, "SYMBOL EXTRA target %s\n", key)
			_, _ = fmt.Fprintf(stdout, "--- %s %s\n%s\n", *leftLabel, key, indent(leftSymbol.text))
		case leftSymbol.text == rightSymbol.text:
			_, _ = fmt.Fprintf(stdout, "SYMBOL OK %s\n", key)
		default:
			_, _ = fmt.Fprintf(stdout, "SYMBOL DIFF %s\n", key)
			printLineDiff(stdout, *leftLabel+" "+key, leftSymbol.text, *rightLabel+" "+key, rightSymbol.text)
		}
	}
	return 0
}

func symbols(path string) (map[string]symbol, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	out := map[string]symbol{}
	order := 0
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			order++
			key := "func " + decl.Name.Name
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				key = "method " + receiverName(decl.Recv.List[0].Type) + "." + decl.Name.Name
			}
			out[key] = symbol{key: key, order: order, text: nodeText(fileSet, decl)}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				order++
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					key := "type " + spec.Name.Name
					out[key] = symbol{key: key, order: order, text: genSpecText(fileSet, decl.Tok.String(), spec)}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						key := decl.Tok.String() + " " + name.Name
						out[key] = symbol{key: key, order: order, text: genSpecText(fileSet, decl.Tok.String(), spec)}
					}
				}
			}
		}
	}
	return out, nil
}

func receiverName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		return "*" + receiverName(expr.X)
	case *ast.IndexExpr:
		return receiverName(expr.X)
	case *ast.IndexListExpr:
		return receiverName(expr.X)
	default:
		return exprText(expr)
	}
}

func genSpecText(fileSet *token.FileSet, tokenName string, spec ast.Spec) string {
	return tokenName + " " + nodeText(fileSet, spec)
}

func nodeText(fileSet *token.FileSet, node any) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, fileSet, node); err != nil {
		return exprText(node)
	}
	return strings.TrimSpace(buf.String())
}

func exprText(node any) string {
	var buf bytes.Buffer
	_ = format.Node(&buf, token.NewFileSet(), node)
	return strings.TrimSpace(buf.String())
}

func indent(value string) string {
	lines := strings.Split(strings.TrimRight(value, "\n"), "\n")
	for i, line := range lines {
		lines[i] = "    " + line
	}
	return strings.Join(lines, "\n")
}

func printLineDiff(writer io.Writer, leftLabel string, left string, rightLabel string, right string) {
	leftLines := strings.Split(strings.TrimRight(left, "\n"), "\n")
	rightLines := strings.Split(strings.TrimRight(right, "\n"), "\n")
	maximum := len(leftLines)
	if len(rightLines) > maximum {
		maximum = len(rightLines)
	}

	_, _ = fmt.Fprintf(writer, "--- %s\n", leftLabel)
	_, _ = fmt.Fprintf(writer, "+++ %s\n", rightLabel)
	for i := 0; i < maximum; i++ {
		var leftLine, rightLine string
		leftOK := i < len(leftLines)
		rightOK := i < len(rightLines)
		if leftOK {
			leftLine = leftLines[i]
		}
		if rightOK {
			rightLine = rightLines[i]
		}
		if leftOK && rightOK && leftLine == rightLine {
			_, _ = fmt.Fprintf(writer, "  %s\n", leftLine)
			continue
		}
		if leftOK {
			_, _ = fmt.Fprintf(writer, "- %s\n", leftLine)
		}
		if rightOK {
			_, _ = fmt.Fprintf(writer, "+ %s\n", rightLine)
		}
	}
}
