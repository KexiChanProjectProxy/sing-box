package log

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var approvedLegacyExceptions = map[string]struct{}{
	// Build/tool CLI commands — not runtime logging, uses package-level log for CLI output
	"cmd/internal/app_store_connect/main.go":       {},
	"cmd/internal/build/main.go":                   {},
	"cmd/internal/build_boxdd/main.go":             {},
	"cmd/internal/build_libbox/main.go":            {},
	"cmd/internal/build_shared/sdk.go":             {},
	"cmd/internal/format_docs/main.go":             {},
	"cmd/internal/merge_aar/main.go":               {},
	"cmd/internal/merge_apple_xcframework/main.go": {},
	"cmd/internal/protogen/main.go":                {},
	"cmd/internal/read_tag/main.go":                {},
	"cmd/internal/tun_bench/main.go":               {},
	"cmd/internal/update_android_version/main.go":  {},
	"cmd/internal/update_apple_version/main.go":    {},
	"cmd/internal/update_certificates/main.go":     {},
	"cmd/internal/update_desktop_version/main.go":  {},
	"constant/goos/gengoos.go":                     {},
	"debug_http.go":                                {},
}

func TestGuardrail(t *testing.T) {
	_, currentFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(currentFile))
	legacyMethods := map[string]bool{"Trace": true, "Debug": true, "Info": true, "Warn": true, "Error": true, "Fatal": true, "Panic": true, "TraceContext": true, "DebugContext": true, "InfoContext": true, "WarnContext": true, "ErrorContext": true, "FatalContext": true, "PanicContext": true}
	eventMethods := map[string]bool{"TraceEvent": true, "DebugEvent": true, "InfoEvent": true, "WarnEvent": true, "ErrorEvent": true, "FatalEvent": true, "PanicEvent": true, "TraceEventContext": true, "DebugEventContext": true, "InfoEventContext": true, "WarnEventContext": true, "ErrorEventContext": true, "FatalEventContext": true, "PanicEventContext": true}
	var violations []string
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if strings.HasPrefix(relSlash, "log/") {
			return nil
		}
		if _, approved := approvedLegacyExceptions[relSlash]; approved {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			position := fset.Position(selector.Pos())
			loc := filepath.ToSlash(rel) + ":" + strconvItoa(position.Line) + ":" + selector.Sel.Name
			if legacyMethods[selector.Sel.Name] && isLegacyLoggerReceiver(selector.X) {
				violations = append(violations, loc)
			}
			if eventMethods[selector.Sel.Name] {
				eventArg, messageArg := eventCallArgs(selector.Sel.Name, call.Args)
				if eventName, ok := stringLiteral(eventArg); ok && strings.HasSuffix(eventName, ".message") {
					violations = append(violations, loc+":event "+eventName)
				}
				if isForbiddenMessageCall(messageArg) {
					violations = append(violations, loc+":interpolated message")
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("legacy logger calls must migrate to structured *Event APIs or be added to approvedLegacyExceptions with rationale:\n%s", strings.Join(violations, "\n"))
	}
}

func isLegacyLoggerReceiver(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name == "log" || x.Name == "logger" || strings.HasSuffix(x.Name, "Logger") || strings.HasSuffix(x.Name, "logger")
	case *ast.SelectorExpr:
		return x.Sel.Name == "logger" || x.Sel.Name == "Logger" || strings.HasSuffix(x.Sel.Name, "logger") || strings.HasSuffix(x.Sel.Name, "Logger")
	default:
		return false
	}
}

func eventCallArgs(method string, args []ast.Expr) (eventArg ast.Expr, messageArg ast.Expr) {
	if strings.HasSuffix(method, "EventContext") {
		if len(args) >= 3 {
			return args[1], args[2]
		}
		return nil, nil
	}
	if len(args) >= 2 {
		return args[0], args[1]
	}
	return nil, nil
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconvUnquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

func isForbiddenMessageCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "F" && selector.Sel.Name == "ToString" || ident.Name == "fmt" && selector.Sel.Name == "Sprintf"
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}

func strconvUnquote(value string) (string, error) {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1], nil
	}
	if len(value) >= 2 && value[0] == '`' && value[len(value)-1] == '`' {
		return value[1 : len(value)-1], nil
	}
	return "", errUnquote
}

type unquoteError struct{}

func (unquoteError) Error() string { return "unquote" }

var errUnquote unquoteError
