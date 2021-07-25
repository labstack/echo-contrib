package oidcdiscovery

func sliceContains(required interface{}, received interface{}) bool {
	switch required.(type) {
	case []string:
		for _, requiredValue := range required.([]string) {
			if !stringSliceContains(received.([]string), requiredValue) {
				return false
			}
		}
	case []int:
		for _, requiredValue := range required.([]int) {
			if !intSliceContains(received.([]int), requiredValue) {
				return false
			}
		}
	case []int8:
		for _, requiredValue := range required.([]int8) {
			if !int8SliceContains(received.([]int8), requiredValue) {
				return false
			}
		}
	case []int16:
		for _, requiredValue := range required.([]int16) {
			if !int16SliceContains(received.([]int16), requiredValue) {
				return false
			}
		}
	case []int32:
		for _, requiredValue := range required.([]int32) {
			if !int32SliceContains(received.([]int32), requiredValue) {
				return false
			}
		}
	case []int64:
		for _, requiredValue := range required.([]int64) {
			if !int64SliceContains(received.([]int64), requiredValue) {
				return false
			}
		}
	case []float32:
		for _, requiredValue := range required.([]float32) {
			if !float32SliceContains(received.([]float32), requiredValue) {
				return false
			}
		}
	case []float64:
		for _, requiredValue := range required.([]float64) {
			if !float64SliceContains(received.([]float64), requiredValue) {
				return false
			}
		}
	}

	return true
}

func stringSliceContains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func intSliceContains(s []int, d int) bool {
	for _, v := range s {
		if v == d {
			return true
		}
	}

	return false
}

func int8SliceContains(s []int8, d int8) bool {
	for _, v := range s {
		if v == d {
			return true
		}
	}

	return false
}

func int16SliceContains(s []int16, d int16) bool {
	for _, v := range s {
		if v == d {
			return true
		}
	}

	return false
}

func int32SliceContains(s []int32, d int32) bool {
	for _, v := range s {
		if v == d {
			return true
		}
	}

	return false
}

func int64SliceContains(s []int64, d int64) bool {
	for _, v := range s {
		if v == d {
			return true
		}
	}

	return false
}

func float32SliceContains(s []float32, d float32) bool {
	for _, v := range s {
		if v == d {
			return true
		}
	}

	return false
}

func float64SliceContains(s []float64, d float64) bool {
	for _, v := range s {
		if v == d {
			return true
		}
	}

	return false
}
