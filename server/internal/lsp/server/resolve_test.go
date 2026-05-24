package server

import (
	"fmt"
	"testing"
)

func TestResolveStdlibFromBinary(t *testing.T) {
	result := resolveStdlibFromBinary("c3c")
	fmt.Printf("resolveStdlibFromBinary(\"c3c\") = %q\n", result)
	
	result2 := resolveStdlibFromBinary("/opt/homebrew/bin/c3c")
	fmt.Printf("resolveStdlibFromBinary(\"/opt/homebrew/bin/c3c\") = %q\n", result2)
}
