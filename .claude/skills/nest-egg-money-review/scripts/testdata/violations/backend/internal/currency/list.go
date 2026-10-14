package currency

var supported = []string{"USD", "JPY", "EUR"}

func valid(code string) bool {
	switch code {
	case "USD", "JPY":
		return true
	}
	return false
}

var method = "GET"
