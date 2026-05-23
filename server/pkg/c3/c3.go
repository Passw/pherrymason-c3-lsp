package c3

// ref: https://c3-lang.org/implementation-details/grammar/#keywords
var keywords = map[string]struct{}{
	"void": {}, "bool": {}, "char": {}, "double": {},
	"float": {}, "float16": {}, "int128": {}, "ichar": {},
	"int": {}, "iptr": {}, "sz": {}, "long": {},
	"short": {}, "uint128": {}, "uint": {}, "ulong": {},
	"uptr": {}, "ushort": {}, "float128": {}, "bfloat16": {},
	"any": {}, "fault": {}, "typeid": {}, "assert": {},
	"asm": {}, "bitstruct": {}, "break": {}, "case": {},
	"catch": {}, "const": {}, "continue": {}, "alias": {},
	"default": {}, "defer": {}, "typedef": {}, "do": {},
	"else": {}, "enum": {}, "extern": {}, "false": {},
	"for": {}, "foreach": {}, "foreach_r": {}, "fn": {},
	"tlocal": {}, "if": {}, "inline": {}, "import": {},
	"macro": {}, "module": {}, "nextcase": {}, "null": {},
	"return": {}, "static": {}, "struct": {}, "switch": {},
	"true": {}, "try": {}, "union": {}, "var": {},
	"while": {}, "attrdef": {}, "constdef": {}, "faultdef": {},

	"$assert": {}, "$case": {}, "$default": {},
	"$defined": {}, "$echo": {}, "$embed": {}, "$exec": {},
	"$else": {}, "$endfor": {}, "$endforeach": {}, "$endif": {},
	"$endswitch": {}, "$eval": {}, "$error": {},
	"$extnameof": {}, "$for": {}, "$foreach": {}, "$if": {},
	"$include": {}, "$qnameof": {}, "$expand": {},
	"$stringify": {}, "$switch": {}, "$Typefrom": {},
	"$Typeof": {}, "$vacount": {}, "$vatype": {}, "$vaconst": {},
	"$vaarg": {}, "$vaexpr": {}, "$vasplat": {}, "$feature": {},
}

func Keywords() map[string]struct{} {
	return keywords
}

func IsLanguageKeyword(symbol string) bool {
	_, exists := keywords[symbol]
	return exists
}
