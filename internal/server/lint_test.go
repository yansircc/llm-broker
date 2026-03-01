package server

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoAdHocMapInWriteJSON uses AST parsing to ensure no writeJSON call
// passes a map[string]interface{} literal. This prevents regressions where
// ad-hoc maps bypass the DTO contract.
func TestNoAdHocMapInWriteJSON(t *testing.T) {
	fset := token.NewFileSet()

	// Parse all .go files in this package (excluding tests)
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		// Fallback: parse by glob
		files, _ := filepath.Glob("*.go")
		if len(files) == 0 {
			t.Skip("cannot find source files")
		}
		pkgs = make(map[string]*ast.Package)
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			af, parseErr := parser.ParseFile(fset, f, nil, parser.ParseComments)
			if parseErr != nil {
				t.Fatalf("parse %s: %v", f, parseErr)
			}
			pkg, ok := pkgs[af.Name.Name]
			if !ok {
				pkg = &ast.Package{Name: af.Name.Name, Files: make(map[string]*ast.File)}
				pkgs[af.Name.Name] = pkg
			}
			pkg.Files[f] = af
		}
	}

	for _, pkg := range pkgs {
		for filename, file := range pkg.Files {
			if strings.HasSuffix(filename, "_test.go") {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				// Match writeJSON(...)
				ident, ok := call.Fun.(*ast.Ident)
				if !ok || ident.Name != "writeJSON" {
					return true
				}

				// writeJSON(w, status, v) — check 3rd arg
				if len(call.Args) < 3 {
					return true
				}

				arg := call.Args[2]

				// Check for lint:allow-map-response comment on the line
				pos := fset.Position(call.Pos())
				if hasAllowComment(file, fset, pos.Line) {
					return true
				}

				if isMapStringInterface(arg) {
					t.Errorf("%s:%d: writeJSON uses map[string]interface{} literal — use a named DTO struct instead",
						filename, pos.Line)
				}

				return true
			})
		}
	}
}

// isMapStringInterface checks if an expression is a map[string]interface{} composite literal.
func isMapStringInterface(expr ast.Expr) bool {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return false
	}
	mt, ok := lit.Type.(*ast.MapType)
	if !ok {
		return false
	}
	// key must be "string"
	keyIdent, ok := mt.Key.(*ast.Ident)
	if !ok || keyIdent.Name != "string" {
		return false
	}
	// value must be "interface{}" (represented as *ast.InterfaceType with no methods)
	iface, ok := mt.Value.(*ast.InterfaceType)
	if !ok {
		return false
	}
	return iface.Methods == nil || len(iface.Methods.List) == 0
}

// hasAllowComment checks if there's a "lint:allow-map-response" comment on the given line.
func hasAllowComment(file *ast.File, fset *token.FileSet, line int) bool {
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			cPos := fset.Position(c.Pos())
			if cPos.Line == line && strings.Contains(c.Text, "lint:allow-map-response") {
				return true
			}
		}
	}
	return false
}
