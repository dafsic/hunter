package utils

// 获取列表最大值
func GetMaxOfList(val []float64) (max float64) {
	max = val[0]
	for _, v := range val {
		if v > max {
			max = v
		}
	}
	return
}

// 获取列表最小值
func GetMinOfList(val []float64) (min float64) {
	min = val[0]
	for _, v := range val {
		if v < min {
			min = v
		}
	}
	return
}

// 求列表的平均值
func GetMeanOfList(val []float64) (mean float64) {
	sum := 0.0
	for _, v := range val {
		sum += v
	}
	mean = sum / float64(len(val))
	return
}

// 获取列表的和
func GetSumOfLise(val []int) (sum int) {
	sum = 0
	for _, v := range val {
		sum += v
	}
	return
}

// 列表求交集
func Intersection(list1, list2 []string) []string {
	set := make(map[string]bool)
	intersect := []string{}

	for _, s := range list1 {
		set[s] = true
	}

	for _, s := range list2 {
		if set[s] {
			intersect = append(intersect, s)
		}
	}

	return intersect
}

// 一维数组转二维数组 size为二维数组中一维数组的长度
func Arr2Arr[T any](arr []T, size int) [][]T {

	if len(arr) < size {
		return [][]T{arr}
	}
	length := len(arr) / size
	slices := make([][]T, length+1)
	i := 0
	for {
		if len(arr)-size < 1 {
			slices[i] = arr
			break
		}
		slices[i] = arr[:size]
		i++
		arr = arr[size:]
	}
	return slices
}
