package colors

const (
	red    = "\033[0;31m"
	yellow = "\033[0;33m"
	reset  = "\033[0m"
)

// Red adds terminal codes make text appear red.
func Red(s string) string {
	return red + s + reset
}

// Yellow adds terminal codes make text appear yellow.
func Yellow(s string) string {
	return yellow + s + reset
}
