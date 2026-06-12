package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
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
	leftLabel := flag.String("left-label", "left", "label for the left file")
	rightLabel := flag.String("right-label", "right", "label for the right file")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: go-symbol-diff [--left-label label] [--right-label label] LEFT.go RIGHT.go")
		os.Exit(2)
	}

	left, err := symbols(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse %s: %v\n", flag.Arg(0), err)
		os.Exit(2)
	}
	right, err := symbols(flag.Arg(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse %s: %v\n", flag.Arg(1), err)
		os.Exit(2)
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
			fmt.Printf("SYMBOL MISSING target %s\n", key)
			fmt.Printf("+++ %s %s\n%s\n", *rightLabel, key, indent(rightSymbol.text))
		case !rightOK:
			fmt.Printf("SYMBOL EXTRA target %s\n", key)
			fmt.Printf("--- %s %s\n%s\n", *leftLabel, key, indent(leftSymbol.text))
		case leftSymbol.text == rightSymbol.text:
			fmt.Printf("SYMBOL OK %s\n", key)
		default:
			fmt.Printf("SYMBOL DIFF %s\n", key)
			printLineDiff(*leftLabel+" "+key, leftSymbol.text, *rightLabel+" "+key, rightSymbol.text)
		}
	}
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

func printLineDiff(leftLabel string, left string, rightLabel string, right string) {
	leftLines := strings.Split(strings.TrimRight(left, "\n"), "\n")
	rightLines := strings.Split(strings.TrimRight(right, "\n"), "\n")
	maximum := len(leftLines)
	if len(rightLines) > maximum {
		maximum = len(rightLines)
	}

	fmt.Printf("--- %s\n", leftLabel)
	fmt.Printf("+++ %s\n", rightLabel)
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
			fmt.Printf("  %s\n", leftLine)
			continue
		}
		if leftOK {
			fmt.Printf("- %s\n", leftLine)
		}
		if rightOK {
			fmt.Printf("+ %s\n", rightLine)
		}
	}
}
