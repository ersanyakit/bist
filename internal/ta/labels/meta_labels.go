package labels

func MetaLabel(ret, threshold float64) int {
	if DirectionLabel(ret, threshold) == "flat" {
		return 0
	}
	if ret > 0 {
		return 1
	}
	return 0
}
