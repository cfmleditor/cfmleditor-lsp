package docs

import "strings"

// BuiltinObject defines the methods available on an object returned by a built-in function.
type BuiltinObject struct {
	Methods []string
}

// builtinReturnObjects maps built-in CF function names to their return object methods.
var builtinReturnObjects = map[string]*BuiltinObject{
	// File object (from fileOpen)
	"fileopen": {Methods: []string{
		"close", "readline", "readbinary", "read", "write", "writeline",
		"seek", "iseof", "setencoding", "getpath",
	}},
	// Spreadsheet object
	"spreadsheetnew": {Methods: []string{
		"addrow", "addrows", "addcolumn", "addimage", "addinfo", "addfreezepane",
		"addpagebreaks", "addsheet", "addsplitpane",
		"deleterow", "deleterows", "deletecolumn", "deletecolumns",
		"formatrow", "formatrows", "formatcolumn", "formatcolumns", "formatcell", "formatcellrange",
		"getcellvalue", "getcellformula", "setcellvalue", "setcellformula", "setcellcomment",
		"getcolumncount", "getrowcount",
		"mergecells", "setactivesheet", "setactivesheetnumber",
		"setcolumnwidth", "setrowheight", "setheader", "setfooter",
		"shiftrows", "shiftcolumns",
		"removesheet", "renamesheet",
		"write",
	}},
	"spreadsheetread": {Methods: []string{
		"addrow", "addrows", "addcolumn", "addimage", "addinfo", "addfreezepane",
		"addpagebreaks", "addsheet", "addsplitpane",
		"deleterow", "deleterows", "deletecolumn", "deletecolumns",
		"formatrow", "formatrows", "formatcolumn", "formatcolumns", "formatcell", "formatcellrange",
		"getcellvalue", "getcellformula", "setcellvalue", "setcellformula", "setcellcomment",
		"getcolumncount", "getrowcount",
		"mergecells", "setactivesheet", "setactivesheetnumber",
		"setcolumnwidth", "setrowheight", "setheader", "setfooter",
		"shiftrows", "shiftcolumns",
		"removesheet", "renamesheet",
		"write",
	}},
	// Image object
	"imageread": {Methods: []string{
		"resize", "scaletofit", "crop", "rotate", "flip", "transpose",
		"blur", "sharpen", "grayscale", "negative", "overlay",
		"setdrawingcolor", "setdrawingstroke", "setdrawingtransparency",
		"drawtext", "drawline", "drawlines", "drawpoint",
		"drawrect", "drawroundrect", "drawoval", "drawarc", "drawcubiccurve", "drawquadraticcurve",
		"getwidth", "getheight", "getexifmetadata", "getiptcmetadata",
		"info", "paste", "translate", "writeto", "write",
		"addborder", "setantialiasing", "setbackgroundcolor",
	}},
	"imagenew": {Methods: []string{
		"resize", "scaletofit", "crop", "rotate", "flip", "transpose",
		"blur", "sharpen", "grayscale", "negative", "overlay",
		"setdrawingcolor", "setdrawingstroke", "setdrawingtransparency",
		"drawtext", "drawline", "drawlines", "drawpoint",
		"drawrect", "drawroundrect", "drawoval", "drawarc", "drawcubiccurve", "drawquadraticcurve",
		"getwidth", "getheight", "getexifmetadata", "getiptcmetadata",
		"info", "paste", "translate", "writeto", "write",
		"addborder", "setantialiasing", "setbackgroundcolor",
	}},
	"imagecaptcha": {Methods: []string{
		"write", "writeto",
	}},
	// Query object
	"querynew": {Methods: []string{
		"addrow", "addcolumn", "deleterow", "deletecolumn",
		"setcell", "getrow", "sort", "filter", "reduce", "map", "each",
		"recordcount", "columnlist", "getcolumnnames",
		"getcell", "columnexists",
	}},
	"queryexecute": {Methods: []string{
		"addrow", "addcolumn", "deleterow", "deletecolumn",
		"setcell", "getrow", "sort", "filter", "reduce", "map", "each",
		"recordcount", "columnlist", "getcolumnnames",
		"getcell", "columnexists",
	}},
	// Thread object (via cfthread)
	"threadnew": {Methods: []string{
		"join", "terminate", "getoutput", "getstatus",
	}},
	// HTTP response
	"httpservice": {Methods: []string{
		"send", "setmethod", "seturl", "addparam",
		"getprefix", "setresolveurls", "settimeout",
	}},
	// XML object
	"xmlparse": {Methods: []string{
		"search", "transform", "tostring", "getnode",
		"xmlroot", "xmlchildren", "xmlattributes", "xmltext",
	}},
	"xmlnew": {Methods: []string{
		"search", "transform", "tostring", "getnode",
		"xmlroot", "xmlchildren", "xmlattributes", "xmltext",
	}},
}

// LookupBuiltinReturnComponent returns a synthetic component name for a built-in
// function's return value, or empty string if not mapped.
func LookupBuiltinReturnComponent(funcName string) string {
	if _, ok := builtinReturnObjects[strings.ToLower(funcName)]; ok {
		return "$builtin." + strings.ToLower(funcName)
	}

	return ""
}

// LookupBuiltinMethod returns true if the given function returns an object
// that has the specified method.
func LookupBuiltinMethod(funcName, method string) bool {
	obj := builtinReturnObjects[strings.ToLower(funcName)]
	if obj == nil {
		return false
	}

	m := strings.ToLower(method)
	for _, name := range obj.Methods {
		if name == m {
			return true
		}
	}

	return false
}
