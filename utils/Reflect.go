package utils

import "reflect"

func GetStructName(i interface{}) string {
	t := reflect.TypeOf(i)
	if t.Kind() == reflect.Ptr {
		t = t.Elem() // Dereference pointer to get the actual type
	}
	return t.Name()
}
