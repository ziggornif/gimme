package array

// ArrayContains - Return if the array contains the input value
func ArrayContains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
