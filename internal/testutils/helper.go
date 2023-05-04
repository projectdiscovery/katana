package testutils

func CompareOutput(input, expected []string) bool {
	if len(input) != len(expected) {
		return false
	}
	for i, v := range input {
		if v != expected[i] {
			return false
		}
	}
	return true
}
