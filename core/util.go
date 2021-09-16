package autospotting

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func itemInSlice(search string, items []string) bool {
	for _, item := range items {
		if search == item {
			return true
		}
	}
	return false
}
