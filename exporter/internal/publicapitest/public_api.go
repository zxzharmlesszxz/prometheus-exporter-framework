package publicapitest

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

type Options struct {
	Dir                 string
	GoldenPath          string
	Update              bool
	UpdateCommand       string
	ReadDirLabel        string
	ModulePath          string
	AliasInternalPrefix string
}

func Check(t TB, options Options) {
	t.Helper()

	if options.Dir == "" {
		options.Dir = "."
	}
	if options.GoldenPath == "" {
		options.GoldenPath = "testdata/public_api.txt"
	}

	got := Collect(t, options)
	gotBytes := []byte(strings.Join(got, "\n") + "\n")

	if options.Update {
		if err := os.MkdirAll(filepath.Dir(options.GoldenPath), 0o755); err != nil {
			t.Fatalf("create testdata dir: %v", err)
		}
		if err := os.WriteFile(options.GoldenPath, gotBytes, 0o644); err != nil {
			t.Fatalf("update golden file: %v", err)
		}
	}

	wantBytes, err := os.ReadFile(options.GoldenPath)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}

	if !bytes.Equal(gotBytes, wantBytes) {
		updateCommand := options.UpdateCommand
		if updateCommand == "" {
			updateCommand = "go test -update-public-api"
		}
		t.Fatalf("public API surface changed; run `%s` if this change is intentional\n\nwant:\n%s\n\ngot:\n%s", updateCommand, wantBytes, gotBytes)
	}
}

func Collect(t TB, options Options) []string {
	t.Helper()

	entries, err := os.ReadDir(options.Dir)
	if err != nil {
		label := options.ReadDirLabel
		if label == "" {
			label = options.Dir
		}
		t.Fatalf("read %s dir: %v", label, err)
	}

	fset := token.NewFileSet()
	var symbols []string

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		filePath := filepath.Join(options.Dir, name)
		file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", filePath, err)
		}

		imports := fileImports(file)
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil && decl.Name.IsExported() {
					symbols = append(symbols, "func "+decl.Name.Name+" "+formatSignature(t, fset, decl.Type))
				} else if decl.Recv != nil && decl.Name.IsExported() && exportedReceiverName(decl.Recv.List[0].Type) != "" {
					symbols = append(symbols, "method ("+formatNode(t, fset, decl.Recv.List[0].Type)+") "+decl.Name.Name+" "+formatSignature(t, fset, decl.Type))
				}

			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						if spec.Name.IsExported() {
							typeName := exportedTypeDeclName(t, fset, spec)
							if spec.Assign.IsValid() {
								symbols = append(symbols, "type "+typeName+" = "+formatNode(t, fset, spec.Type))
								symbols = append(symbols, exportedAliasTargetMembers(t, fset, options, imports, spec)...)
							} else if isStructOrInterface(spec.Type) {
								symbols = append(symbols, "type "+typeName)
							} else {
								symbols = append(symbols, "type "+typeName+" "+formatNode(t, fset, spec.Type))
							}
							symbols = append(symbols, exportedTypeMembers(t, fset, spec)...)
						}

					case *ast.ValueSpec:
						for _, ident := range spec.Names {
							if ident.IsExported() {
								symbols = append(symbols, decl.Tok.String()+" "+ident.Name)
							}
						}
					}
				}
			}
		}
	}

	sort.Strings(symbols)

	return symbols
}

func exportedTypeDeclName(t TB, fset *token.FileSet, spec *ast.TypeSpec) string {
	t.Helper()

	name := spec.Name.Name
	if spec.TypeParams != nil {
		name += formatTypeParams(t, fset, spec.TypeParams)
	}
	return name
}

func formatTypeParams(t TB, fset *token.FileSet, params *ast.FieldList) string {
	t.Helper()

	if params == nil || len(params.List) == 0 {
		return ""
	}
	parts := make([]string, 0, len(params.List))
	for _, field := range params.List {
		names := make([]string, 0, len(field.Names))
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
		if len(names) == 0 {
			parts = append(parts, formatNode(t, fset, field.Type))
			continue
		}
		parts = append(parts, strings.Join(names, ", ")+" "+formatNode(t, fset, field.Type))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func isStructOrInterface(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.StructType, *ast.InterfaceType:
		return true
	default:
		return false
	}
}

func fileImports(file *ast.File) map[string]string {
	imports := make(map[string]string, len(file.Imports))
	for _, imported := range file.Imports {
		importPath := strings.Trim(imported.Path.Value, `"`)
		name := path.Base(importPath)
		if imported.Name != nil {
			name = imported.Name.Name
		}
		imports[name] = importPath
	}
	return imports
}

func exportedAliasTargetMembers(t TB, fset *token.FileSet, options Options, imports map[string]string, spec *ast.TypeSpec) []string {
	t.Helper()

	if options.ModulePath == "" || options.AliasInternalPrefix == "" {
		return nil
	}
	pkgName, typeName := aliasTargetSelector(spec.Type)
	if pkgName == "" || typeName == "" {
		return nil
	}
	importPath := imports[pkgName]
	if !strings.HasPrefix(importPath, options.AliasInternalPrefix) {
		return nil
	}

	targetDir := filepath.Join(options.Dir, filepath.FromSlash(strings.TrimPrefix(importPath, options.ModulePath+"/exporter/")))
	targetSpec, targetMethods := targetTypeSpecAndMethods(t, fset, targetDir, typeName)
	if targetSpec == nil {
		return nil
	}

	var members []string
	for _, member := range exportedTypeMembers(t, fset, targetSpec) {
		members = append(members, replaceAliasMemberName(member, typeName, spec.Name.Name))
	}
	for _, method := range targetMethods {
		members = append(members, replaceAliasMemberName(method, typeName, spec.Name.Name))
	}
	for i, member := range members {
		switch {
		case strings.HasPrefix(member, "field "):
			members[i] = "alias-" + member
		case strings.HasPrefix(member, "interface "):
			members[i] = "alias-" + member
		case strings.HasPrefix(member, "method "):
			members[i] = "alias-" + member
		}
	}
	return members
}

func replaceAliasMemberName(member string, targetName string, aliasName string) string {
	member = strings.Replace(member, " "+targetName+".", " "+aliasName+".", 1)
	member = strings.Replace(member, "(*"+targetName, "(*"+aliasName, 1)
	member = strings.Replace(member, "("+targetName, "("+aliasName, 1)
	return member
}

func aliasTargetSelector(expr ast.Expr) (string, string) {
	switch expr := expr.(type) {
	case *ast.SelectorExpr:
		pkg, ok := expr.X.(*ast.Ident)
		if !ok {
			return "", ""
		}
		return pkg.Name, expr.Sel.Name
	case *ast.IndexExpr:
		return aliasTargetSelector(expr.X)
	case *ast.IndexListExpr:
		return aliasTargetSelector(expr.X)
	}
	return "", ""
}

func targetTypeSpecAndMethods(t TB, fset *token.FileSet, dir string, typeName string) (*ast.TypeSpec, []string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read alias target dir %s: %v", dir, err)
	}

	var targetSpec *ast.TypeSpec
	var methods []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		filePath := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse alias target %s: %v", filePath, err)
		}

		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv != nil && decl.Name.IsExported() && exportedReceiverName(decl.Recv.List[0].Type) == typeName {
					methods = append(methods, "method ("+formatNode(t, fset, decl.Recv.List[0].Type)+") "+decl.Name.Name+" "+formatSignature(t, fset, decl.Type))
				}

			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if ok && typeSpec.Name.Name == typeName {
						targetSpec = typeSpec
					}
				}
			}
		}
	}
	return targetSpec, methods
}

func exportedTypeMembers(t TB, fset *token.FileSet, spec *ast.TypeSpec) []string {
	t.Helper()

	switch typ := spec.Type.(type) {
	case *ast.StructType:
		return exportedStructFields(t, fset, spec.Name.Name, typ)
	case *ast.InterfaceType:
		return exportedInterfaceMethods(t, fset, spec.Name.Name, typ)
	default:
		return nil
	}
}

func exportedStructFields(t TB, fset *token.FileSet, typeName string, typ *ast.StructType) []string {
	t.Helper()

	var fields []string
	for _, field := range typ.Fields.List {
		fieldType := formatNode(t, fset, field.Type)
		if field.Tag != nil {
			fieldType += " " + field.Tag.Value
		}
		if len(field.Names) == 0 {
			if name := exportedTypeName(field.Type); name != "" {
				fields = append(fields, "field "+typeName+"."+name+" "+fieldType)
			}
			continue
		}
		for _, name := range field.Names {
			if name.IsExported() {
				fields = append(fields, "field "+typeName+"."+name.Name+" "+fieldType)
			}
		}
	}
	return fields
}

func exportedInterfaceMethods(t TB, fset *token.FileSet, typeName string, typ *ast.InterfaceType) []string {
	t.Helper()

	var methods []string
	for _, field := range typ.Methods.List {
		methodType := formatNode(t, fset, field.Type)
		if len(field.Names) == 0 {
			if name := exportedTypeName(field.Type); name != "" {
				methods = append(methods, "interface "+typeName+"."+name+" "+methodType)
			}
			continue
		}
		for _, name := range field.Names {
			if name.IsExported() {
				methods = append(methods, "interface "+typeName+"."+name.Name+" "+methodType)
			}
		}
	}
	return methods
}

func exportedReceiverName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		if expr.IsExported() {
			return expr.Name
		}
	case *ast.StarExpr:
		return exportedReceiverName(expr.X)
	case *ast.IndexExpr:
		return exportedReceiverName(expr.X)
	case *ast.IndexListExpr:
		return exportedReceiverName(expr.X)
	}

	return ""
}

func exportedTypeName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		if expr.IsExported() {
			return expr.Name
		}
	case *ast.SelectorExpr:
		if expr.Sel.IsExported() {
			return expr.Sel.Name
		}
	case *ast.StarExpr:
		return exportedTypeName(expr.X)
	case *ast.IndexExpr:
		return exportedTypeName(expr.X)
	case *ast.IndexListExpr:
		return exportedTypeName(expr.X)
	}

	return ""
}

func formatNode(t TB, fset *token.FileSet, node ast.Node) string {
	t.Helper()

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		t.Fatalf("format AST node: %v", err)
	}

	return buf.String()
}

func formatSignature(t TB, fset *token.FileSet, node ast.Node) string {
	t.Helper()

	return strings.Join(strings.Fields(formatNode(t, fset, node)), " ")
}
