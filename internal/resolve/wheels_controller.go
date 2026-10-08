package resolve

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

// Controller integrates controller/view packages after its inherited methods.
// A normal method never replaces an own method; super<name> is the original
// framework method only when an actual own/inherited override is present.
func (r *Resolver) wheelsControllerFunc(chain []string, name string) *parser.FunctionDef {
	for at, path := range chain {
		if !strings.EqualFold(filepath.Base(path), "Controller.cfc") {
			continue
		}

		if !r.wheelsControllerIntegrates(path) {
			continue
		}

		if !r.wheelsControllerInheritsLoader(chain[:at]) || !r.wheelsControllerExtendsGlobal(chain, at) {
			return nil
		}

		lookup := name

		alias := strings.HasPrefix(strings.ToLower(name), "super")
		if alias {
			lookup = name[len("super"):]
			if !r.wheelsOwnFunc(chain, lookup) {
				return nil
			}
		}

		found := r.wheelsControllerPackageFunc(path, lookup)

		if found != nil && alias {
			bound := *found
			bound.Name = name

			return &bound
		}

		return found
	}

	return nil
}

// wheelsControllerIntegrates reports whether path is Wheels' Controller.cfc
// as pinned: an init that integrates the controller and view packages, and
// the loader that copies their public methods in.
func (r *Resolver) wheelsControllerIntegrates(path string) bool {
	source := r.wheelsSource(path)

	return source.methods["init"].body == wheelsTokens(`$integrateComponents("wheels.controller"); $integrateComponents("wheels.view"); return this;`) &&
		source.methods["$integratecomponents"].body == wheelsTokens(`local.plan = $componentIntegrationPlan(arguments.path); local.overrideSet = $mixinOverrideSet("controller"); local.iEnd = ArrayLen(local.plan); for (local.i = 1; local.i <= local.iEnd; local.i++) { $integrateFunctions(local.plan[local.i].publicMethods, local.overrideSet); }`) &&
		source.methods["$integratefunctions"].body == wheelsTokens(wheelsControllerCopy)
}

// wheelsControllerExtendsGlobal reports whether chain[at], the Controller,
// extends wheels.Global holding the integration plan it calls.
func (r *Resolver) wheelsControllerExtendsGlobal(chain []string, at int) bool {
	global := r.ComponentPath("wheels.Global", filepath.Dir(chain[at]))

	return global != "" && at+1 < len(chain) && samePath(chain[at+1], global) && r.wheelsIntegrationPlan(global) && r.wheelsPlanFunc(global, "$mixinOverrideSet") != nil
}

// wheelsMixinHostFunc answers a bare call made in one of the components
// Controller.cfc integrates (wheels/controller/*.cfc, wheels/view/*.cfc).
// Such a component is never instantiated; its public methods are copied into
// every controller and run there, so a name it calls is the controller's
// (and wheels/mapper/*.cfc's the Mapper's, wheelsMapperMixinFunc):
// Controller's own, wheels.Global's chain and includes, or another integrated
// package method. Nothing answers unless the Controller beside the package is
// the pinned one and integrates this very file.
func (r *Resolver) wheelsMixinHostFunc(pr *parser.ParseResult, name string) *parser.FunctionDef {
	if !pr.URI.IsFile() || r.fileExtends(pr) != "" {
		return nil
	}

	file := pr.URI.Path()
	dir := filepath.Dir(file)
	pkg := strings.ToLower(filepath.Base(dir))

	if pkg == "mapper" {
		return r.wheelsMapperMixinFunc(file, name)
	}

	if pkg != "controller" && pkg != "view" {
		return nil
	}

	host := filepath.Join(filepath.Dir(dir), "Controller.cfc")
	if !samePath(file, r.ComponentPath("wheels."+pkg+"."+strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)), filepath.Dir(dir))) ||
		!samePath(host, r.ComponentPath("wheels.Controller", filepath.Dir(dir))) || !r.wheelsControllerIntegrates(host) {
		return nil
	}

	chain := []string{host}

	for base, baseDir := r.extendsOfPath(host), filepath.Dir(host); base != "" && len(chain) < 16; {
		next := r.ComponentPath(base, baseDir)
		if next == "" || slices.ContainsFunc(chain, func(p string) bool { return samePath(p, next) }) {
			break
		}

		chain = append(chain, next)
		base, baseDir = r.extendsOfPath(next), filepath.Dir(next)
	}

	if !r.wheelsControllerExtendsGlobal(chain, 0) {
		return nil
	}

	if def := r.lookupFunc(host, name, 0); def != nil {
		return def
	}

	return r.wheelsControllerFunc(chain, name)
}

// wheelsMapperMixinFunc is wheelsMixinHostFunc for wheels/mapper/*.cfc, whose
// public methods Mapper.init copies into itself after wheels.Global's: a bare
// call there is the Mapper's, through the same lookup a call on a Mapper makes.
func (r *Resolver) wheelsMapperMixinFunc(file, name string) *parser.FunctionDef {
	dir := filepath.Dir(file)
	host := filepath.Join(filepath.Dir(dir), "Mapper.cfc")

	if !samePath(file, r.ComponentPath("wheels.mapper."+strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)), filepath.Dir(dir))) ||
		!samePath(host, r.ComponentPath("wheels.Mapper", filepath.Dir(dir))) {
		return nil
	}

	if _, ok := r.wheelsMapperSetup(host); !ok {
		return nil
	}

	return r.lookupFunc(host, name, 0)
}

func (r *Resolver) extendsOfPath(path string) string {
	base, _ := r.extendsOf(path, cfpath.ToURI(path))

	return base
}

func (r *Resolver) wheelsControllerInheritsLoader(descendants []string) bool {
	for _, path := range descendants {
		methods := r.wheelsSource(path).methods
		if _, ok := methods["$integratecomponents"]; ok {
			return false
		}

		if _, ok := methods["$integratefunctions"]; ok {
			return false
		}

		for _, name := range []string{"$componentintegrationplan", "$buildcomponentintegrationplan", "$mixinoverrideset"} {
			if _, ok := methods[name]; ok {
				return false
			}
		}

		if init, ok := methods["init"]; ok && init.body != wheelsTokens(`return super.init();`) && init.body != wheelsTokens(`super.init(); return this;`) {
			return false
		}
	}

	return true
}

func (r *Resolver) wheelsOwnFunc(chain []string, name string) bool {
	for _, path := range chain {
		for _, def := range r.EnsureIndexed(path) {
			if strings.EqualFold(def.Name, name) {
				return true
			}
		}

		if r.includedFunc(path, name) != nil {
			return true
		}
	}

	return false
}

const wheelsControllerCopy = `
 local.iEnd = ArrayLen(arguments.publicMethods);
 for (local.i = 1; local.i <= local.iEnd; local.i++) {
  local.m = arguments.publicMethods[local.i];
  local.name = local.m.name;
  local.ref = local.m.ref;
  if (!(StructKeyExists(variables, local.name) || StructKeyExists(this, local.name))) {
   variables[local.name] = local.ref;
   this[local.name] = local.ref;
  } else {
   local.superName = "super" & local.name;
   variables[local.superName] = local.ref;
   this[local.superName] = local.ref;
  }
  if (StructKeyExists(arguments.overrideSet, local.name)) {
   local.superName = "super" & local.name;
   variables[local.superName] = local.ref;
   this[local.superName] = local.ref;
  }
 }`

// An inherited lookup needs the actual receiving file for generated super
// aliases and initializer overrides. Ordinary inheritance still starts at the
// base; only source-declared Wheels mixin definitions use the complete chain.
func (r *Resolver) inheritedFunc(pr *parser.ParseResult, name, baseDir string) *parser.FunctionDef {
	base := r.fileExtends(pr)

	def := r.ResolveFunc(base, name, baseDir)
	if !pr.URI.IsFile() {
		return def
	}

	chain := []string{pr.URI.Path()}
	seen := map[string]bool{pr.URI.Path(): true}

	for len(chain) < 16 && base != "" {
		path := r.ComponentPath(base, baseDir)
		if path == "" || seen[path] {
			break
		}

		seen[path] = true
		chain = append(chain, path)

		next, ok := r.extendsOf(path, cfpath.ToURI(path))
		if !ok {
			break
		}

		base = next
		baseDir = filepath.Dir(path)
	}

	for _, path := range chain[1:] {
		if !strings.EqualFold(filepath.Base(path), "Controller.cfc") {
			continue
		}

		if r.wheelsSource(path).methods["init"].body != wheelsTokens(`$integrateComponents("wheels.controller"); $integrateComponents("wheels.view"); return this;`) {
			continue
		}

		if def != nil && def.URI.IsFile() {
			dir := filepath.Dir(def.URI.Path())
			if !samePath(dir, filepath.Join(filepath.Dir(path), "controller")) && !samePath(dir, filepath.Join(filepath.Dir(path), "view")) {
				return def
			}
		}

		mixed := r.wheelsControllerFunc(chain, name)
		if mixed != nil {
			return mixed
		}

		if def != nil {
			dir := filepath.Dir(def.URI.Path())
			if samePath(dir, filepath.Join(filepath.Dir(path), "controller")) || samePath(dir, filepath.Join(filepath.Dir(path), "view")) {
				return nil
			}
		}
	}

	return def
}

func (r *Resolver) wheelsControllerPackageFunc(path, lookup string) *parser.FunctionDef {
	var found *parser.FunctionDef

	for _, packageName := range []string{"controller", "view"} {
		entries, err := r.fs().ReadDir(filepath.Join(filepath.Dir(path), packageName))
		if err != nil || len(entries) > 128 {
			return nil
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".cfc") {
				continue
			}

			file := filepath.Join(filepath.Dir(path), packageName, entry.Name())
			if !samePath(file, r.ComponentPath("wheels."+packageName+"."+strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), filepath.Dir(path))) {
				return nil
			}

			method, ok := r.wheelsSource(file).methods[strings.ToLower(lookup)]
			if !ok || !method.public {
				continue
			}

			for _, def := range r.EnsureIndexed(file) {
				if !strings.EqualFold(def.Name, lookup) {
					continue
				}

				if found != nil {
					return nil
				}

				found = def
			}
		}
	}

	return found
}
