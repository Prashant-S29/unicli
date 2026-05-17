package ui

import "fmt"

// Success prints a green success line.
//
//	✓  message
func Success(msg string) {
	fmt.Printf("%s  %s\n", StyleSuccess.Render(SymbolSuccess), msg)
}

// Error prints a red error block with optional reason and fix.
//
//	✗  message
//	   Reason: reason
//	   Fix:    fix
func Error(msg, reason, fix string) {
	fmt.Printf("%s  %s\n", StyleError.Render(SymbolError), StyleBold.Render(msg))
	if reason != "" {
		fmt.Printf("   %s %s\n", StyleLabel.Render("Reason:"), reason)
	}
	if fix != "" {
		fmt.Printf("   %s %s\n", StyleLabel.Render("Fix:"), StyleInfo.Render(fix))
	}
}

// Warning prints an amber warning line.
//
//	!  message
func Warning(msg string) {
	fmt.Printf("%s  %s\n", StyleWarning.Render(SymbolWarning), msg)
}

// Info prints a blue info line.
//
//	→  message
func Info(msg string) {
	fmt.Printf("%s  %s\n", StyleInfo.Render(SymbolInfo), msg)
}

// Muted prints a dimmed line — for secondary detail.
func Muted(msg string) {
	fmt.Println(StyleMuted.Render("   " + msg))
}

// Blank prints an empty line — for visual breathing room.
func Blank() {
	fmt.Println()
}
