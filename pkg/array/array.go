package array

// ArrayContains - Return if the array contains the input value
func ArrayContains[T comparable](arr []T, input T) bool {
	for _, val := range arr {
		if val == input {
			return true
		}
	}

	return false
}
