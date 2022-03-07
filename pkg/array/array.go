package array

// ArrayContains - Return if the array contains the input value
// Could be improved with generics later
// func ArrayContains[T any] (arr []T, input T) bool
func ArrayContains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
