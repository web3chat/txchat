package dao

import "strconv"

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func parseInt8(s string) (int8, error) {
	v, err := strconv.ParseInt(s, 10, 8)
	return int8(v), err
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}
