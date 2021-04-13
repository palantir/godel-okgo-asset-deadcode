package amalgomated

import (
	"github.com/palantir/godel-okgo-asset-deadcode/generated_src/internal/amalgomated_flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var exitCode int

var (
	withTestFiles bool
)

func AmalgomatedMain() {
	flag.BoolVar(&withTestFiles, "test", false, "include test files")
	flag.Parse()
	ctx := &Context{
		withTests: withTestFiles,
	}
	pkgs := []string{"."}
	if flag.NArg() != 0 {
		pkgs = flag.Args()
	}
	ctx.Load(pkgs...)
	report := ctx.Process()
	for _, obj := range report {
		ctx.errorf(obj.fset, obj.obj.Pos(), "%s is unused", obj.obj.Name())
	}
	os.Exit(exitCode)
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

type Context struct {
	cwd		string
	withTests	bool

	loadedPkgs	[]*packages.Package
	loadErr		error
}

func (ctx *Context) Load(args ...string) {
	ctx.loadedPkgs, ctx.loadErr = packages.Load(&packages.Config{
		Mode:	packages.LoadSyntax,
		Tests:	ctx.withTests,
	}, args...)
}

// error formats the error to standard error, adding program
// identification and a newline
func (ctx *Context) errorf(fset *token.FileSet, pos token.Pos, format string, args ...interface{}) {
	if ctx.cwd == "" {
		ctx.cwd, _ = os.Getwd()
	}
	p := fset.Position(pos)
	f, err := filepath.Rel(ctx.cwd, p.Filename)
	if err == nil {
		p.Filename = f
	}
	fmt.Fprintf(os.Stderr, p.String()+": "+format+"\n", args...)
	exitCode = 2
}

func (ctx *Context) Process() []objectWithFset {
	if err := ctx.loadErr; err != nil {
		fatalf("cannot load packages: %s", err)
	}
	ctx.loadedPkgs = deduplicateTestPackages(ctx.loadedPkgs)

	var allUnused []objectWithFset
	for _, pkg := range ctx.loadedPkgs {
		unused := doPackage(pkg)
		allUnused = append(allUnused, unused...)
	}
	sort.Sort(objects(allUnused))
	return allUnused
}

var pkgIDRegexp = regexp.MustCompile(`^([^[ ]+)( \[[^]]+]$)?`)

func deduplicateTestPackages(pkgs []*packages.Package) []*packages.Package {
	// required until https://github.com/golang/go/issues/27910 is resolved.
	// Map from package ID to whether or not it is a test. Depends on the implementation detail of the default Go build
	// system that IDs for test packages are of the form "{{pkgPath}} [{{pkgPath}}.test]".
	pkgIDsWithTests := make(map[string]bool)
	for _, pkg := range pkgs {
		matchParts := pkgIDRegexp.FindStringSubmatch(pkg.ID)
		pkgID := pkg.ID
		isTest := false
		if len(matchParts) == 3 && len(matchParts[2]) > 0 {
			pkgID = matchParts[1]
			isTest = true
		}
		pkgIDsWithTests[pkgID] = pkgIDsWithTests[pkgID] || isTest
	}

	filtered := pkgs[:0]
	for _, pkg := range pkgs {
		if !pkgIDsWithTests[pkg.ID] {
			filtered = append(filtered, pkg)
		}
	}
	return filtered
}

func doPackage(pkg *packages.Package) []objectWithFset {
	used := make(map[types.Object]bool)

	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			obj := pkg.TypesInfo.Uses[id]
			if obj != nil {
				used[obj] = true
			}
			return false
		})
	}

	global := pkg.Types.Scope()
	var unused []objectWithFset
	for _, name := range global.Names() {
		if pkg.Types.Name() == "main" && name == "main" {
			continue
		}
		obj := global.Lookup(name)
		if !used[obj] && (pkg.Types.Name() == "main" || !ast.IsExported(name)) {
			unused = append(unused, objectWithFset{
				obj:	obj,
				fset:	pkg.Fset,
			})
		}
	}
	return unused
}

type objectWithFset struct {
	obj	types.Object
	fset	*token.FileSet
}

type objects []objectWithFset

func (s objects) Len() int		{ return len(s) }
func (s objects) Swap(i, j int)		{ s[i], s[j] = s[j], s[i] }
func (s objects) Less(i, j int) bool	{ return s[i].obj.Pos() < s[j].obj.Pos() }
